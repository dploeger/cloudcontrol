package container

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"io"
	"strings"
)

// generate a platform spec from a platform string.
func platformFromString(value string) *v1.Platform {
	separated := strings.Split(value, "/")
	platform := v1.Platform{
		OS:           separated[0],
		Architecture: separated[1],
	}
	if len(separated) == 3 {
		platform.Variant = separated[2]
	}
	return &platform
}

// generates a list of bindmount-definitions from Bind specs.
func bindsFromContainerBinds(containerBinds []Bind) []string {
	return funk.Map(
		containerBinds,
		func(containerBind Bind) string {
			return fmt.Sprintf("%s:%s", containerBind.Source, containerBind.Target)
		},
	).([]string)
}

type DockerAdapter struct {
	dockerCLI *client.Client
}

func (d DockerAdapter) getClient() client.Client {
	if d.dockerCLI == nil {
		if dockerCli, err := client.NewClientWithOpts(client.FromEnv); err != nil {
			panic(fmt.Sprintf("Can not connect to Docker API: %s", err.Error()))
		} else {
			d.dockerCLI = dockerCli
		}
	}
	return *d.dockerCLI
}

func (d DockerAdapter) FindImage(image string, platform string) error {
	dockerCli := d.getClient()
	localImages, err := dockerCli.ImageList(
		context.Background(),
		types.ImageListOptions{
			Filters: filters.NewArgs(filters.KeyValuePair{
				Key:   "reference",
				Value: image,
			}),
		},
	)

	if err != nil {
		panic(fmt.Sprintf("Can not list local container images: %s", err.Error()))
	}

	if len(localImages) == 0 {
		remoteImages, err := dockerCli.ImageSearch(context.Background(), image, types.ImageSearchOptions{Limit: 1})
		if err != nil {
			panic(fmt.Sprintf("Can not search remote images: %s", err.Error()))
		}
		if len(remoteImages) == 0 {
			return ImageNotFoundError{
				image: image,
			}
		}
		if _, err := dockerCli.ImagePull(context.Background(), image, types.ImagePullOptions{
			Platform: platform,
		}); err != nil {
			return fmt.Errorf("can not pull image %w", err)
		}
	}
	return nil
}

func (d DockerAdapter) StartContainer(
	image string,
	envs []string,
	binds []Bind,
	platform string,
) (string, error) {
	var containerID string
	dockerCli := d.getClient()

	if createBody, err := dockerCli.ContainerCreate(
		context.Background(),
		&container.Config{
			Env:   envs,
			Image: image,
		},
		&container.HostConfig{
			Binds: bindsFromContainerBinds(binds),
		},
		&network.NetworkingConfig{},
		platformFromString(platform),
		"",
	); err != nil {
		return "", fmt.Errorf("can not create goss container: %w", err)
	} else {
		for _, warning := range createBody.Warnings {
			logrus.Warnf("Creating the goss container produced this warning: %s", warning)
		}
		containerID = createBody.ID
	}

	if err := dockerCli.ContainerStart(context.Background(), containerID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("can not start goss container %s: %w", containerID, err)
	}

	return containerID, nil
}

func (d DockerAdapter) StopContainer(containerID string) error {
	dockerCli := d.getClient()
	if err := dockerCli.ContainerRemove(context.Background(), containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return err
	}
	return nil
}

func (d DockerAdapter) RunCommand(containerID string, cmd []string) (string, error) {
	dockerCli := d.getClient()
	var executeID string
	if idResponse, err := dockerCli.ContainerExecCreate(context.Background(), containerID, types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}); err != nil {
		return "", fmt.Errorf("can not create wait exec in goss container %s: %w", containerID, err)
	} else {
		executeID = idResponse.ID
	}

	var waitLogReader *bufio.Reader
	if resp, err := dockerCli.ContainerExecAttach(context.Background(), executeID, types.ExecStartCheck{}); err != nil {
		return "", fmt.Errorf("can not attach to wait in container %s: %w", containerID, err)
	} else {
		waitLogReader = resp.Reader
		defer resp.Close()
	}

	if err := dockerCli.ContainerExecStart(context.Background(), executeID, types.ExecStartCheck{}); err != nil {
		return "", fmt.Errorf("can not start wait in container %s: %w", containerID, err)
	}

	for {
		if execInspect, err := dockerCli.ContainerExecInspect(context.Background(), executeID); err != nil {
			return "", fmt.Errorf("wrror waiting for goss in container %s: %w", containerID, err)
		} else {
			if !execInspect.Running {
				commandLogbuf := new(strings.Builder)
				if _, err := io.Copy(commandLogbuf, waitLogReader); err != nil {
					return "", fmt.Errorf("can not get command logs from reader: %w", err)
				}
				if execInspect.ExitCode != 0 {
					containerLogBuf := new(strings.Builder)
					if logReader, err := dockerCli.ContainerLogs(context.Background(), containerID, types.ContainerLogsOptions{
						ShowStderr: true,
						ShowStdout: true,
					}); err != nil {
						return "", fmt.Errorf("can not get logs of container %s: %w", containerID, err)
					} else {
						if _, err := io.Copy(containerLogBuf, logReader); err != nil {
							return "", fmt.Errorf("can not get container logs from reader: %w", err)
						}
					}

					return "", &RunCommandError{
						ReturnCode:      execInspect.ExitCode,
						CommandOutput:   commandLogbuf.String(),
						ContainerOutput: containerLogBuf.String(),
					}
				}
				return commandLogbuf.String(), nil
			}
		}
	}
}
