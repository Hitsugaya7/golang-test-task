package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"io"
	"os"
)

var containerName = "test-containerName" + uuid.New().String()

func runDocker(cli *client.Client, result chan string, imageName string, bashCommand string) {
	ctx := context.Background()

	fmt.Println("Docker image pull")
	reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	defer reader.Close()
	io.Copy(os.Stdout, reader)

	hostBinding := nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: "8000",
	}
	containerPort, err := nat.NewPort("tcp", "80")
	if err != nil {
		panic("Unable to get the port")
	}
	portBinding := nat.PortMap{containerPort: []nat.PortBinding{hostBinding}}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,

		Tty: true,
	}, &container.HostConfig{
		PortBindings: portBinding,
	}, nil, nil, containerName)
	if err != nil {
		panic(err)
	}

	fmt.Println("Container launching")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	idResponse, err := cli.ContainerExecCreate(ctx, resp.ID, types.ExecConfig{
		Cmd:          []string{"sh", "-c", bashCommand},
		Tty:          true,
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  true,
		Detach:       true,
	})
	fmt.Println("Script runnning")
	attach, err := cli.ContainerExecAttach(ctx, idResponse.ID, types.ExecStartCheck{})
	defer attach.Close()
	if err != nil {
		panic(err)
	}

	for {
		line, err := ReadLine(attach.Reader)

		if err == io.EOF {
			close(result)
			return
		} else if err != nil {
			panic(err)
		}
		result <- string(line)
	}
}
func ReadLine(r io.Reader) (line []byte, err error) {
	b := make([]byte, 1)
	var l int
	for err == nil {
		l, err = r.Read(b)
		if l > 0 {
			if b[0] == '\n' {
				return
			}
			line = append(line, b...)
		}
	}
	return
}

func stopAndRemoveContainer(cli *client.Client) error {
	fmt.Println("Wait, the app is stopping and removing the docker container")
	ctx := context.Background()

	if err := cli.ContainerStop(ctx, containerName, nil); err != nil {
		log.Printf("Unable to stop container %s: %s", containerName, err)
	}

	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	if err := cli.ContainerRemove(ctx, containerName, removeOptions); err != nil {
		log.Printf("Unable to remove container: %s", err)
		return err
	}

	return nil
}
