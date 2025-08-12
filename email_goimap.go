package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
)

type GoImapEmailInterface struct {
	c        *imapclient.Client
	fetchCmd *imapclient.FetchCommand
}

func (self *GoImapEmailInterface) initFetch(lastMessagesCount uint32) error {
	mbox, err := self.c.Select("INBOX", nil).Wait()
	if err != nil {
		return err
	}

	fetchOpts := imap.FetchOptions{Envelope: true, UID: true, RFC822Size: true}
	seqset := imap.SeqSet{}
	var start, stop uint32
	stop = mbox.NumMessages
	if mbox.NumMessages < lastMessagesCount {
		start = 0
	} else {
		start = mbox.NumMessages - lastMessagesCount
	}
	seqset.AddRange(start, stop)
	self.fetchCmd = self.c.Fetch(seqset, &fetchOpts)
	return nil
}

func (self *GoImapEmailInterface) fetchNext() (EmailMetadata, error) {
	pMsg := self.fetchCmd.Next()
	if pMsg == nil {
		closeErr := self.fetchCmd.Close()
		return EmailMetadata{}, errors.Join(closeErr, errors.New("no more messages"))
	}
	msg, err := pMsg.Collect()
	if err != nil {
		closeErr := self.fetchCmd.Close()
		return EmailMetadata{}, errors.Join(err, closeErr, errors.New("msg reading error"))
	}
	return EmailMetadata{subject: msg.Envelope.Subject, uid: uint64(msg.UID), bodyLen: msg.RFC822Size}, nil
}

func (self *GoImapEmailInterface) read(id uint64) string {
	seqSet := imap.UIDSetNum(imap.UID(id))
	bodySection := &imap.FetchItemBodySection{}
	fetchOptions := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}
	fetchCmd := self.c.Fetch(seqSet, fetchOptions)
	defer fetchCmd.Close()

	msg := fetchCmd.Next()
	if msg == nil {
		return "msg receive error"
	}

	msgBuf, err := msg.Collect()
	if err != nil || msgBuf == nil {
		return "msg collect err"
	}

	msgBytes := msgBuf.FindBodySection(bodySection)
	if msgBytes == nil {
		return "msg read errrrrrrrrrrrrrrrrr"
	}

	mr, err := mail.CreateReader(bytes.NewReader(msgBytes))
	if err != nil {
		log.Fatalf("failed to create mail reader: %v", err)
	}

	// Print a few header fields
	// h := mr.Header
	// if date, err := h.Date(); err != nil {
	// 	log.Printf("failed to parse Date header field: %v", err)
	// } else {
	// 	log.Printf("Date: %v", date)
	// }
	// if to, err := h.AddressList("To"); err != nil {
	// 	log.Printf("failed to parse To header field: %v", err)
	// } else {
	// 	log.Printf("To: %v", to)
	// }
	// if subject, err := h.Text("Subject"); err != nil {
	// 	log.Printf("failed to parse Subject header field: %v", err)
	// } else {
	// 	log.Printf("Subject: %v", subject)
	// }

	var sbuf strings.Builder
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("failed to read message part: %v", err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			mediaType, _, _ := h.ContentType()
			log.Println("Content type: ", mediaType)
			if mediaType == "text/plain" {
				b, _ := io.ReadAll(p.Body)
				sbuf.Write(b)
			}
			// case *mail.AttachmentHeader:
			// 	// This is an attachment
			// 	filename, _ := h.Filename()
			// 	log.Printf("Attachment: %v", filename)
		}
	}
	return sbuf.String()
}

func (self *GoImapEmailInterface) Logout() {
	// self.c.Logout()
	self.c.Close()
}

type EmailInterface interface {
	initFetch(lastMessagesCount uint32) error
	fetchNext() (EmailMetadata, error)
	read(id uint64) string
}

type GoImapUpdatesNotifier struct {
	reader EmailInterface
}

func (s *GoImapUpdatesNotifier) notify(knownMessages []EmailMetadata, newMessages chan<- EmailMetadata, removedMessages chan<- EmailMetadata) error {
	removedMessagesByUids := make(map[uint64]EmailMetadata)
	for _, v := range knownMessages {
		removedMessagesByUids[v.uid] = v
	}
	err := s.reader.initFetch(100)
	if err != nil {
		return err
	}

	go func() {
		for emailsMetadata, err := s.reader.fetchNext(); err == nil; emailsMetadata, err = s.reader.fetchNext() {
			_, known := removedMessagesByUids[emailsMetadata.uid]
			if known {
				delete(removedMessagesByUids, emailsMetadata.uid)
			} else {
				newMessages <- emailsMetadata
			}
		}
		for _, v := range removedMessagesByUids {
			removedMessages <- v
		}
	}()
	return nil
}

func NewGoImapUpdatesNotifier(reader EmailInterface) *GoImapUpdatesNotifier {
	return &GoImapUpdatesNotifier{reader}
}

type GoImapEmailReader struct {
	emailInterface EmailInterface
}

func (s *GoImapEmailReader) read(id uint64) string {
	return s.emailInterface.read(id)
}
func NewGoImapEmailReader(emailInterface EmailInterface) *GoImapEmailReader {
	return &GoImapEmailReader{emailInterface}
}
