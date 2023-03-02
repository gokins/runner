package runners

import "github.com/gokins/core/runtime"

type ServerInfo struct {
	WebHost   string
	DownToken string
}
type ReqPullJob struct {
	Name  string
	Plugs []string
}
type UpdateJobInfo struct {
	BuildId  string
	JobId    string
	Status   string
	Error    string
	ExitCode int
}
type RunJob struct {
	Id           string
	PipelineId   string
	StageId      string
	BuildId      string
	StageName    string
	Step         string
	Name         string
	Input        map[string]string
	Env          map[string]string
	OriginRepo   string
	UsersRepo    string
	Commands     []*CmdContent
	Artifacts    []*runtime.Artifact
	UseArtifacts []*runtime.UseArtifact
}
type CmdContent struct {
	Id string `json:"id"`
	//Gid string `json:"gid"`
	//Pid   string    `json:"pid"`
	Conts string `json:"conts"`
	//Times time.Time `json:"times"`
}

type DirEntry struct {
	Name  string
	IsDir bool
	Size  int64
}

type FileStat struct {
	Name  string
	IsDir bool
	Size  int64
}
