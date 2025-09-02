package main

import (
	"bytes"
	"errors"
	"fmt"
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
	var subject string
	if msg.Envelope != nil {
		subject = msg.Envelope.Subject
	}
	return EmailMetadata{subject: subject, uid: uint64(msg.UID), bodyLen: msg.RFC822Size}, nil
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

func (self *GoImapEmailInterface) remove(id uint64) error {
	uidSet := imap.UIDSetNum(imap.UID(id))

	// Gmail-specific deletion: Move to Trash folder instead of marking as deleted
	// This is the proper way to delete emails in Gmail via IMAP
	log.Printf("Moving message UID %d to Trash folder", id)

	// Try to move to Gmail's Trash folder
	trashFolder := "[Gmail]/Trash"
	moveCmd := self.c.Move(uidSet, trashFolder)
	if _, err := moveCmd.Wait(); err != nil {
		log.Printf("Failed to move to %s, trying alternative folder names: %v", trashFolder, err)

		// Try alternative Trash folder names that Gmail might use
		alternativeNames := []string{"[Google Mail]/Trash", "Trash", "[Gmail]/Bin", "[Google Mail]/Bin"}

		moveSucceeded := false
		for _, folder := range alternativeNames {
			log.Printf("Attempting to move to folder: %s", folder)
			moveCmd := self.c.Move(uidSet, folder)
			if _, err := moveCmd.Wait(); err != nil {
				log.Printf("Failed to move to %s: %v", folder, err)
				continue
			}
			log.Printf("Successfully moved message to %s", folder)
			moveSucceeded = true
			break
		}

		if moveSucceeded {
			return nil
		}

		// If all move attempts failed, fall back to the traditional delete + expunge method
		log.Printf("All move attempts failed, falling back to delete+expunge method")

		storeFlags := &imap.StoreFlags{
			Op:    imap.StoreFlagsAdd,
			Flags: []imap.Flag{imap.FlagDeleted},
		}

		if err := self.c.Store(uidSet, storeFlags, nil); err != nil {
			return fmt.Errorf("failed to mark message as deleted: %v", err)
		}

		if err := self.c.Expunge(); err != nil {
			return fmt.Errorf("failed to expunge deleted message: %v", err)
		}

		log.Printf("Message marked as deleted and expunged (may still be in All Mail)")
		return nil
	}

	log.Printf("Successfully moved message to Trash folder")
	return nil

	// seqSet := imap.UIDSetNum(imap.UID(id))

	// storeFlags := &imap.StoreFlags{
	// 	Op:    imap.StoreFlagsAdd,
	// 	Flags: []imap.Flag{imap.FlagDeleted},
	// }

	// if err := self.c.Store(seqSet, storeFlags, nil); err != nil {
	// 	return fmt.Errorf("failed to mark message as deleted: %v", err)
	// }

	// if err := self.c.Expunge(); err != nil {
	// 	return fmt.Errorf("failed to expunge deleted message: %v", err)
	// }

	// return nil
}

type EmailInterface interface {
	initFetch(lastMessagesCount uint32) error
	fetchNext() (EmailMetadata, error)
	read(id uint64) string
	remove(id uint64) error
}

type GoImapUpdatesNotifier struct {
	reader EmailInterface
}

func (s *GoImapUpdatesNotifier) notify(knownMessages []EmailMetadata, newMessages chan<- EmailMetadata, removedMessages chan<- EmailMetadata) {
	removedMessagesByUids := make(map[uint64]EmailMetadata)
	for _, v := range knownMessages {
		removedMessagesByUids[v.uid] = v
	}
	err := s.reader.initFetch(100)
	if err != nil {
		log.Fatalf("failed to init fetch: %v", err)
	}

	for emailsMetadata, err := s.reader.fetchNext(); err == nil; emailsMetadata, err = s.reader.fetchNext() {
		if emailsMetadata.bodyLen == 0 {
			continue
		}
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
