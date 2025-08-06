package main

import (
	"slices"
	"testing"

	"github.com/winfsp/cgofuse/fuse"
)

type FakeUpdatesNotifier struct {
	metadata        []EmailMetadata
	newMessages     chan<- EmailMetadata
	removedMessages chan<- EmailMetadata
}

func (s *FakeUpdatesNotifier) notify(knownMessages []EmailMetadata, newMessages chan<- EmailMetadata, removedMessages chan<- EmailMetadata) error {
	s.newMessages = newMessages
	s.removedMessages = removedMessages
	return nil
}

type FakeEmailReader struct {
	body string
}

func (s *FakeEmailReader) read(id uint64) string {
	return s.body
}

func TestReaddir(t *testing.T) {
	subjects := []string{"email 1 subject", "email 2 subject"}
	testMetadata := []EmailMetadata{
		{subject: subjects[0]},
		{subject: subjects[1]},
	}

	emailNotifier := FakeUpdatesNotifier{}
	fs := EmailFs{emailNotifier: &emailNotifier}
	fs.Init()

	var dirItems []string
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		dirItems = append(dirItems, name)
		return true
	}
	for _, v := range testMetadata {
		emailNotifier.newMessages <- v
	}
	fs.Readdir("/", fill, 0, 0)

	slices.Sort(subjects)
	slices.Sort(dirItems)
	if slices.Compare(subjects, dirItems) != 0 {
		t.Errorf("Exp %s got %s", subjects, dirItems)
	}
}

func TestRead(t *testing.T) {
	body := "bodyyyyyyyyyyy"
	testMetadata := EmailMetadata{
		subject: "mail1-subject", uid: uint64(5), bodyLen: int64(len(body)),
	}
	filename := "/" + testMetadata.subject

	emailReader := FakeEmailReader{}
	emailNotifier := FakeUpdatesNotifier{}
	fs := EmailFs{emailReader: &emailReader, emailNotifier: &emailNotifier, userId: 1000}
	fs.Init()

	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		return true
	}
	emailNotifier.newMessages <- testMetadata
	emailReader.body = body
	fs.Readdir("/", fill, 0, 0)
	fs.Open(filename, 0)
	buf := make([]byte, 99)
	lenRead := fs.Read(filename, buf, 0, 0)
	if string(buf[:lenRead]) != body {
		t.Errorf("Exp %s got %s", body, string(buf))
	}
}
