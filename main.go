package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/winfsp/cgofuse/fuse"
)

func main() {
	user, _ := user.Current()
	userId64, _ := strconv.ParseUint(user.Uid, 10, 16)
	userId := uint(userId64)
	godotenv.Load()

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	gmailTokenFilepath := filepath.Join(exeDir, "gmail-token.json")
	emailAuth, err := NewGAuth(gmailTokenFilepath)
	emailInterface, err := emailAuth.Login()
	if err != nil {
		log.Fatalln(err)
	}
	defer emailAuth.Logout()

	emailNotifier := NewGoImapUpdatesNotifier(emailInterface)
	emailReader := NewGoImapEmailReader(emailInterface)
	hellofs := &EmailFs{
		emailNotifier:  emailNotifier,
		emailReader:    emailReader,
		userId:         userId,
		updateInterval: time.Minute * 1,
	}
	host := fuse.NewFileSystemHost(hellofs)
	args, err := parseArgs()
	if err != nil {
		printUsage()
		os.Exit(1)
	}
	host.Mount(args.mountpoint, nil)
}

type argsStruct struct {
	mountpoint string
}

func parseArgs() (argsStruct, error) {
	if len(os.Args) != 2 {
		return argsStruct{}, errors.New("wrong usage")
	}
	return argsStruct{mountpoint: os.Args[1]}, nil
}

func printUsage() {
	fmt.Println("Usage: emailfs <mountpoint>")
}
