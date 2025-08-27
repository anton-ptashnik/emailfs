package main

import (
	"fmt"
	"log"
	"time"

	"github.com/winfsp/cgofuse/fuse"
)

type EmailMetadata struct {
	uid     uint64
	subject string
	bodyLen int64
}

type EmailReader interface {
	read(id uint64) string
}

type EmailUpdatesNotifier interface {
	notify(knownMessages []EmailMetadata, newMessages chan<- EmailMetadata, removedMessages chan<- EmailMetadata)
}

type TimerStarter func() <-chan time.Time

type EmailFs struct {
	fuse.FileSystemBase
	emailReader         EmailReader
	emailNotifier       EmailUpdatesNotifier
	emailsMetadata      map[string]EmailMetadata
	openFiles           map[uint64]string
	userId              uint
	newMessages         chan EmailMetadata
	removedMessages     chan EmailMetadata
	updateIntervalTimer TimerStarter
}

func (self *EmailFs) Init() {
	self.openFiles = make(map[uint64]string)
	self.emailsMetadata = make(map[string]EmailMetadata)
	self.newMessages = make(chan EmailMetadata, 500)
	self.removedMessages = make(chan EmailMetadata, 500)

	go func() {
		for {
			// todo refactor
			var currentMetadata []EmailMetadata
			for _, v := range self.emailsMetadata {
				currentMetadata = append(currentMetadata, v)
			}
			self.emailNotifier.notify(currentMetadata, self.newMessages, self.removedMessages)
			<-self.updateIntervalTimer()
			self.fetchUpdates()
		}
	}()
}

func (self *EmailFs) Destroy() {}

func (self *EmailFs) Open(path string, flags int) (errc int, fh uint64) {
	// if match == nil {
	// 	return -fuse.ENOENT, ^uint64(0)
	// }
	log.Printf("Open file %s\n", path)
	uid := self.emailsMetadata[path].uid
	body := self.emailReader.read(uid)
	self.openFiles[uid] = body
	return 0, uid
}

func (self *EmailFs) Release(path string, fh uint64) int {
	log.Printf("Release file %s\n", path)
	delete(self.openFiles, fh)
	return 0
}

func (self *EmailFs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	stat.Uid = uint32(self.userId)
	stat.Gid = stat.Uid
	if path == "/" {
		stat.Mode = fuse.S_IFDIR | 0550
		return 0
	}

	log.Printf("Getattr %s\n", path)
	stat.Mode = fuse.S_IFREG | 0440
	stat.Size = int64(self.emailsMetadata[path].bodyLen)
	stat.Blocks = (stat.Size + 511) / 512
	return 0
}

func (self *EmailFs) Read(path string, buff []byte, ofst int64, fh uint64) int {
	log.Printf("Read file: %s , handle: %d", path, fh)
	endofst := ofst + int64(len(buff))
	contents := self.openFiles[fh]
	if endofst > int64(len(contents)) {
		endofst = int64(len(contents))
	}
	if endofst < ofst {
		return 0
	}
	return copy(buff, contents[ofst:endofst])
}

func (self *EmailFs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	log.Println("readdir, offs: ", ofst)
	// fill(".", &fuse.Stat_t{Mode: syscall.S_IFDIR | 0755}, 1)
	// fill("..", &fuse.Stat_t{Mode: syscall.S_IFDIR | 0755}, 2)

	self.fetchUpdates()

	var stat fuse.Stat_t
	stat.Mode = fuse.S_IFREG | 0440
	for _, email := range self.emailsMetadata {
		stat.Size = int64(email.bodyLen)
		stat.Blocks = (stat.Size + 511) / 512
		fillOk := fill(email.subject, &stat, 0) //int64(len(self.emailsMetadata)))
		if !fillOk {
			errc = 1
			break
		}
	}
	return
}

func (self *EmailFs) fetchUpdates() {
	for more := true; more; {
		select {
		case email := <-self.newMessages:
			email.subject = ClearFilename(email.subject)
			self.emailsMetadata[fmt.Sprintf("/%s", email.subject)] = email
		case email := <-self.removedMessages:
			delete(self.emailsMetadata, fmt.Sprintf("/%s", email.subject))
		default:
			more = false
		}
	}
}
