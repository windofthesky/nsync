package recipebuilder

import (
	"fmt"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/diego-ssh/keys"
	fileserver "github.com/cloudfoundry-incubator/file-server"
	"github.com/cloudfoundry-incubator/runtime-schema/cc_messages"
	"github.com/cloudfoundry/gunk/urljoiner"
)

const (
	MinCpuProxy = 256
	MaxCpuProxy = 8192

	DefaultFileDescriptorLimit = uint64(1024)

	LRPLogSource    = "CELL"
	AppLogSource    = "APP"
	HealthLogSource = "HEALTH"

	Router      = "router"
	DefaultPort = uint32(8080)

	DefaultSSHPort = uint32(2222)

	DefaultLANG = "en_US.UTF-8"
)

var (
	ErrNoDockerImage        = Error{Type: "ErrNoDockerImage", Message: "no docker image provided"}
	ErrNoLifecycleDefined   = Error{Type: "ErrNoLifecycleDefined", Message: "no lifecycle binary bundle defined for stack"}
	ErrDropletSourceMissing = Error{Type: "ErrAppSourceMissing", Message: "desired app missing droplet_uri"}
	ErrDockerImageMissing   = Error{Type: "ErrDockerImageMissing", Message: "desired app missing docker_image"}
	ErrMultipleAppSources   = Error{Type: "ErrMultipleAppSources", Message: "desired app contains both droplet_uri and docker_image; exactly one is required."}
)

type Config struct {
	Lifecycles    map[string]string
	FileServerURL string
	KeyFactory    keys.SSHKeyFactory
}

//go:generate counterfeiter -o ../bulk/fakes/fake_recipe_builder.go . RecipeBuilder
type RecipeBuilder interface {
	Build(*cc_messages.DesireAppRequestFromCC) (*models.DesiredLRP, error)
	ExtractExposedPorts(*cc_messages.DesireAppRequestFromCC) ([]uint32, error)
}

type Error struct {
	Type    string `json:"name"`
	Message string `json:"message"`
}

func (err Error) Error() string {
	return err.Message
}

func lifecycleDownloadURL(lifecyclePath string, fileServerURL string) string {
	staticPath, err := fileserver.Routes.CreatePathForRoute(fileserver.StaticRoute, nil)
	if err != nil {
		panic("couldn't generate the download path for the bundle of app lifecycle binaries: " + err.Error())
	}

	return urljoiner.Join(fileServerURL, staticPath, lifecyclePath)
}

func cpuWeight(memoryMB int) uint32 {
	cpuProxy := memoryMB

	if cpuProxy > MaxCpuProxy {
		return 100
	}

	if cpuProxy < MinCpuProxy {
		return 1
	}

	return uint32(99.0*(cpuProxy-MinCpuProxy)/(MaxCpuProxy-MinCpuProxy) + 1)
}

func createLrpEnv(env []*models.EnvironmentVariable, exposedPort uint32) []*models.EnvironmentVariable {
	env = append(env, &models.EnvironmentVariable{Name: "PORT", Value: fmt.Sprintf("%d", exposedPort)})
	return env
}

func getParallelAction(ports []uint32, user string) *models.ParallelAction {
	fileDescriptorLimit := DefaultFileDescriptorLimit
	parallelAction := &models.ParallelAction{}
	for _, port := range ports {
		parallelAction.Actions = append(parallelAction.Actions,
			&models.Action{
				RunAction: &models.RunAction{
					User:      user,
					Path:      "/tmp/lifecycle/healthcheck",
					Args:      []string{fmt.Sprintf("-port=%d", port)},
					LogSource: HealthLogSource,
					ResourceLimits: &models.ResourceLimits{
						Nofile: &fileDescriptorLimit,
					},
					SuppressLogOutput: true,
				},
			})
	}
	return parallelAction
}

func getDesiredAppPorts(ports []uint32) []uint32 {
	desiredAppPorts := ports

	if desiredAppPorts == nil {
		desiredAppPorts = []uint32{DefaultPort}
	}
	return desiredAppPorts
}
