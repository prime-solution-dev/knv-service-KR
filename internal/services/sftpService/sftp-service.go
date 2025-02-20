package sftpService

import (
	"fmt"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func NewClient() (*sftp.Client, *ssh.Client, error) {
	sshHost := "61.91.5.96"
	sshPort := "22"
	sshUsername := "mboxuser01"
	sshPassword := "M897@pAt_Bb45#qAy" //todo: อย่าลืมมาแก้ hard code

	sshConfig := &ssh.ClientConfig{
		User: sshUsername,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshConn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", sshHost, sshPort), sshConfig)
	if err != nil {
		return nil, nil, err
	}

	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return nil, nil, err
	}

	return sftpClient, sshConn, nil
}
