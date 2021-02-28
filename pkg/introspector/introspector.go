package introspector

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	ds "github.com/goldfiglabs/rpcheckup/pkg/dockersession"
	ps "github.com/goldfiglabs/rpcheckup/pkg/postgres"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const introspectorRef = "goldfig/introspector:v1"
const introspectorContainerName = "introspector"

// Service is a wrapper around a docker container running
// https://github.com/goldfiglabs/introspector.
type Service struct {
	ds.ContainerService
}

func New(s *ds.Session, postgresService ps.PostgresService) (*Service, error) {
	log.Info("Checking for introspector image")
	err := s.RequireImage(introspectorRef)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get instrospector docker image")
	}
	service, err := createIntrospectorContainer(s, postgresService)
	if err != nil {
		return nil, err
	}
	err = s.Client.ContainerStart(s.Ctx, service.ContainerID, types.ContainerStartOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to start introspector")
	}
	log.Info("Initializing introspector")
	err = service.runCommand([]string{"init"}, nil, true)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init introspector")
	}
	return service, nil
}

func (i *Service) ImportAWSService(environmentCredentials []string, serviceSpec string) error {
	return i.runCommand(
		[]string{"account", "aws", "import", "--force", "--service", serviceSpec}, environmentCredentials, true)
}

func createIntrospectorContainer(s *ds.Session, postgresService ps.PostgresService) (*Service, error) {
	existingContainer, err := s.FindContainer(introspectorContainerName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list existing containers")
	}
	if existingContainer != nil {
		err = s.StopAndRemoveContainer(existingContainer.ID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to remove existing container")
		}
	}

	credential := postgresService.SuperUserCredential()
	address := postgresService.Address()
	envVars := []string{
		fmt.Sprintf("INTROSPECTOR_SU_DB_USER=%v", credential.Username),
		fmt.Sprintf("INTROSPECTOR_SU_DB_PASSWORD=%v", credential.Password),
		fmt.Sprintf("INTROSPECTOR_DB_HOST=%v", address.HostIP),
		fmt.Sprintf("INTROSPECTOR_DB_PORT=%v", address.HostPort),
	}
	log.Infof("Using environment %v", envVars)
	containerBody, err := s.Client.ContainerCreate(s.Ctx, &container.Config{
		Image: introspectorRef,
		Env:   envVars,
	}, &container.HostConfig{
		NetworkMode: "host",
	}, &network.NetworkingConfig{}, nil, introspectorContainerName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create container")
	}
	log.Infof("introspector container ID %v", containerBody.ID)
	return &Service{
		ds.ContainerService{ContainerID: containerBody.ID, DockerSession: s},
	}, nil
}

type logWriter struct {
	fn func(args ...interface{})
}

func (l *logWriter) Write(p []byte) (int, error) {
	l.fn(string(p))
	return len(p), nil
}

func (i *Service) runCommand(args []string, env []string, logIntrospector bool) error {
	envVars := []string{}
	if env != nil {
		envVars = append(envVars, env...)
	}
	cmdPrefix := []string{"python", "introspector.py"}
	cmd := append(cmdPrefix, args...)
	execResp, err := i.DockerSession.Client.ContainerExecCreate(i.DockerSession.Ctx, i.ContainerID, types.ExecConfig{
		Cmd:          cmd,
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  true,
		Env:          envVars,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to create exec")
	}
	resp, err := i.DockerSession.Client.ContainerExecAttach(i.DockerSession.Ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return errors.Wrap(err, "Failed to attach to exec")
	}
	defer resp.Close()

	outputDone := make(chan error)
	if logIntrospector {
		errWriter := logWriter{log.Error}
		infoWriter := logWriter{log.Info}
		go func() {
			_, err = stdcopy.StdCopy(&infoWriter, &errWriter, resp.Reader)
			outputDone <- err
		}()
	}

	select {
	case err := <-outputDone:
		if err != nil {
			return err
		}
		break

	case <-i.DockerSession.Ctx.Done():
		return i.DockerSession.Ctx.Err()
	}

	return nil
}
