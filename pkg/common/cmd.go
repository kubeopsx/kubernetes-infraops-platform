package common

import (
	"bytes"
	"context"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

func GetHostName() string {
	name, _ := os.Hostname()
	return name
}

func GetLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		log.Printf("get local addr err:%v", err)
		return ""
	}
	localIp := strings.Split(conn.LocalAddr().String(), ":")[0]
	defer conn.Close()
	return localIp
}

func GetCommand(cmdStr string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	var bufBytes bytes.Buffer
	cmd.Stdout = &bufBytes
	cmd.Stderr = &bufBytes
	if err := cmd.Start(); err != nil {
		return bufBytes.String(), err
	}
	if err := cmd.Wait(); err != nil {
		return bufBytes.String(), err
	}
	return bufBytes.String(), nil
}
