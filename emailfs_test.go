package main

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/winfsp/cgofuse/fuse"
)

type FakeUpdatesNotifier struct {
	metadata         []EmailMetadata
	newMessages      chan<- EmailMetadata
	removedMessages  chan<- EmailMetadata
	knownMessages    []EmailMetadata
	notifyCalledChan chan bool
}

func (s *FakeUpdatesNotifier) notify(knownMessages []EmailMetadata, newMessages chan<- EmailMetadata, removedMessages chan<- EmailMetadata) {
	s.newMessages = newMessages
	s.removedMessages = removedMessages
	s.knownMessages = knownMessages
	s.notifyCalledChan <- true
}

func NewFakeUpdatesNotifier() *FakeUpdatesNotifier {
	return &FakeUpdatesNotifier{notifyCalledChan: make(chan bool)}
}

type FakeEmailReader struct {
	body string
}

func (s *FakeEmailReader) read(id uint64) string {
	return s.body
}

type FakeEmailRemover struct {
	retErr error
}

func (s *FakeEmailRemover) remove(id uint64) error {
	return s.retErr
}

func NewFakeEmailRemover(retErr error) *FakeEmailRemover {
	return &FakeEmailRemover{retErr: retErr}
}

func TestReaddir(t *testing.T) {
	var subjects []string
	for i := 0; i < 100; i++ {
		subjects = append(subjects, fmt.Sprintf("email subject %d", i))
	}

	emailNotifier := NewFakeUpdatesNotifier()
	fs := EmailFs{emailNotifier: emailNotifier, updateIntervalTimer: createNeverTickUpdateIntervalTimer}
	fs.Init()

	<-emailNotifier.notifyCalledChan

	var dirItems []string
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		dirItems = append(dirItems, name)
		return true
	}
	for _, v := range subjects {
		emailNotifier.newMessages <- EmailMetadata{subject: v}
	}

	fs.Readdir("/", fill, 0, 0)

	if !checkSubjectsMatch(subjects, dirItems) {
		t.Errorf("Exp %s got %s", subjects, dirItems)
	}
}

func TestReaddirFailsOnProhibitedFilename(t *testing.T) {
	var subjects []string
	for i := 0; i < 5; i++ {
		subjects = append(subjects, fmt.Sprintf("email subject %d", i))
	}

	emailNotifier := NewFakeUpdatesNotifier()
	fs := EmailFs{emailNotifier: emailNotifier, updateIntervalTimer: createNeverTickUpdateIntervalTimer}
	fs.Init()

	<-emailNotifier.notifyCalledChan

	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		return false
	}
	for _, v := range subjects {
		emailNotifier.newMessages <- EmailMetadata{subject: v}
	}

	errCode := fs.Readdir("/", fill, 0, 0)
	if errCode != 1 {
		t.Errorf("Received %d errc instead of 1", errCode)
	}
}

func TestRead(t *testing.T) {
	body := "bodyyyyyyyyyyy"
	testMetadata := EmailMetadata{
		subject: "mail1-subject", uid: uint64(5), bodyLen: int64(len(body)),
	}
	filename := "/" + testMetadata.subject

	emailReader := FakeEmailReader{}
	emailNotifier := NewFakeUpdatesNotifier()
	fs := EmailFs{emailReader: &emailReader, emailNotifier: emailNotifier, userId: 1000, updateIntervalTimer: createNeverTickUpdateIntervalTimer}
	fs.Init()

	<-emailNotifier.notifyCalledChan

	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		return true
	}
	emailNotifier.newMessages <- testMetadata
	emailReader.body = body
	fs.Readdir("/", fill, 0, 0)
	_, fh := fs.Open(filename, 0)
	buf := make([]byte, 99)
	lenRead := fs.Read(filename, buf, 0, fh)
	if string(buf[:lenRead]) != body {
		t.Errorf("Exp %s got %s", body, string(buf))
	}
}

func TestReaddirIncludesEmailUpdates(t *testing.T) {
	var testSubjects []string
	for i := 0; i < 100; i++ {
		testSubjects = append(testSubjects, fmt.Sprintf("email subject %d", i))
	}

	emailNotifier := NewFakeUpdatesNotifier()
	fs := EmailFs{emailNotifier: emailNotifier, updateIntervalTimer: createNeverTickUpdateIntervalTimer}
	fs.Init()

	<-emailNotifier.notifyCalledChan

	if emailNotifier.knownMessages != nil {
		t.Errorf("Exp knownMessages=nil on Init, got %v", emailNotifier.knownMessages)
	}

	var listedDirItems []string
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		listedDirItems = append(listedDirItems, name)
		return true
	}
	for _, v := range testSubjects {
		emailNotifier.newMessages <- EmailMetadata{subject: v}
	}
	fs.Readdir("/", fill, 0, 0)

	listedDirItems = []string{}
	removedEmailSubject := testSubjects[0]
	testSubjects = testSubjects[1:]
	emailNotifier.removedMessages <- EmailMetadata{subject: removedEmailSubject}
	fs.Readdir("/", fill, 0, 0)

	if !checkSubjectsMatch(testSubjects, listedDirItems) {
		t.Errorf("Exp %s got %s", testSubjects, listedDirItems)
	}

	listedDirItems = []string{}
	addedEmailSubhect := "new email"
	emailNotifier.newMessages <- EmailMetadata{subject: addedEmailSubhect}
	testSubjects = append(testSubjects, addedEmailSubhect)
	fs.Readdir("/", fill, 0, 0)

	if !checkSubjectsMatch(testSubjects, listedDirItems) {
		t.Errorf("Exp %s got %s", testSubjects, listedDirItems)
	}
}

func TestEmailUpdatesArePeriodicallyFetched(t *testing.T) {
	emailNotifier := NewFakeUpdatesNotifier()
	updateIntervalTick := make(chan time.Time)
	fs := EmailFs{emailNotifier: emailNotifier, updateIntervalTimer: func() <-chan time.Time {
		return updateIntervalTick
	}}

	fs.Init()
	var testSubjects []string

	for i := 0; i < 10; i++ {
		select {
		case <-emailNotifier.notifyCalledChan:
		case <-time.After(time.Second * 2):
			t.Fatalf("Timeout waiting for notify to be called: expected 10 calls, got %d", i)
		}

		knownSubjects := []string{}
		for _, v := range emailNotifier.knownMessages {
			knownSubjects = append(knownSubjects, v.subject)
		}
		if !checkSubjectsMatch(testSubjects, knownSubjects) {
			t.Errorf("Exp %s got %s", testSubjects, knownSubjects)
		}

		testSubjects = append(testSubjects, fmt.Sprintf("email subject %d", i))
		emailNotifier.newMessages <- EmailMetadata{subject: testSubjects[i]}
		updateIntervalTick <- time.Now()
	}
}

func TestEmailRemoval(t *testing.T) {
	emailRemover := NewFakeEmailRemover(nil)
	emailNotifier := NewFakeUpdatesNotifier()
	subjects := []string{"email subject 1", "email subject 2", "email subject 3"}
	fs := EmailFs{emailRemover: emailRemover, updateIntervalTimer: createNeverTickUpdateIntervalTimer, emailNotifier: emailNotifier}
	fs.Init()

	<-emailNotifier.notifyCalledChan

	var dirItems []string
	fill := func(name string, stat *fuse.Stat_t, ofst int64) bool {
		dirItems = append(dirItems, name)
		return true
	}
	for i, v := range subjects {
		emailNotifier.newMessages <- EmailMetadata{subject: v, uid: uint64(i)}
	}

	fs.Readdir("/", fill, 0, 0)

	if !checkSubjectsMatch(subjects, dirItems) {
		t.Errorf("Exp %s got %s", subjects, dirItems)
	}

	errCode := fs.Unlink("/" + subjects[0])
	if errCode != 0 {
		t.Errorf("Received %d errc instead of 0", errCode)
	}

	subjects = subjects[1:]
	dirItems = dirItems[:0]
	fs.Readdir("/", fill, 0, 0)
	if !checkSubjectsMatch(subjects, dirItems) {
		t.Errorf("Exp %s got %s", subjects, dirItems)
	}
}

func checkSubjectsMatch(submittedSubjects []string, listedSubjects []string) bool {
	slices.Sort(submittedSubjects)
	slices.Sort(listedSubjects)
	return slices.Compare(submittedSubjects, listedSubjects) == 0
}

func createNeverTickUpdateIntervalTimer() <-chan time.Time {
	return make(chan time.Time)
}
