package dockersession

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type Session struct {
	Ctx    context.Context
	Client *client.Client
}

func NewSession() (*Session, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}
	ctx := context.Background()
	return &Session{
		Ctx:    ctx,
		Client: cli,
	}, nil
}

func (ds *Session) StopAndRemoveContainer(id string) error {
	err := ds.Client.ContainerStop(ds.Ctx, id, nil)
	if err != nil {
		errors.WithMessage(err, "Failed to stop container")
	}
	err = ds.Client.ContainerRemove(ds.Ctx, id, types.ContainerRemoveOptions{
		RemoveVolumes: false,
	})
	if err != nil {
		return errors.WithMessage(err, "Failed to remove container")
	}
	return nil
}

func (ds *Session) FindContainer(name string) (*types.Container, error) {
	existingContainers, err := ds.Client.ContainerList(ds.Ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "/"+name)),
		All:     true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list existing containers")
	}
	if len(existingContainers) > 1 {
		return nil, errors.New(fmt.Sprintf("More than one container found with name %v", name))
	}
	if len(existingContainers) == 1 {
		return &existingContainers[0], nil
	}
	return nil, nil
}

func (ds *Session) RequireImage(ref string) error {
	images, err := ds.Client.ImageList(ds.Ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", ref)),
	})
	if err != nil {
		return errors.WithMessage(err, "Failed to list images")
	}
	if len(images) == 0 {
		log.Infof("Image %v not found, pulling", ref)
		closer, err := ds.Client.ImagePull(ds.Ctx, ref, types.ImagePullOptions{})
		if err != nil {
			return errors.WithMessage(err, "Failed to pull image")
		}
		buf := new(strings.Builder)
		_, err = io.Copy(buf, closer)
		if err != nil {
			return err
		}
		log.Debug(buf.String())
		closer.Close()
	}
	return nil
}

type ContainerService struct {
	ContainerID   string
	DockerSession *Session
}

func (cs *ContainerService) ShutDown() error {
	return cs.DockerSession.StopAndRemoveContainer(cs.ContainerID)
}
