package main

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/winfsp/cgofuse/fuse"
)

func main() {
	user, _ := user.Current()
	uid64, _ := strconv.ParseUint(user.Uid, 10, 16)
	uid := uint(uid64)
	godotenv.Load()

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	gmailTokenFilepath := filepath.Join(exeDir, "token.json")
	emailAuth, err := NewGAuth(gmailTokenFilepath)
	emailInterface, err := emailAuth.Login()
	if err != nil {
		log.Fatalln(err)
	}
	defer emailAuth.Logout()

	emailNotifier := NewGoImapUpdatesNotifier(emailInterface)
	emailReader := NewGoImapEmailReader(emailInterface)
	hellofs := &EmailFs{emailNotifier: emailNotifier, emailReader: emailReader, userId: uid}
	host := fuse.NewFileSystemHost(hellofs)
	host.Mount("./vfs", os.Args[1:])
}
