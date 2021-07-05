package runners

import (
	"encoding/json"
	"github.com/gokins-main/core/utils"
	"os"
	"strconv"
	"time"
)

const childcmd = "commandChildProcess"
const childprefix = "*******--------*******:"

func init() {
	if len(os.Args) >= 4 && os.Args[1] == childcmd {
		os.Exit(childProcess())
		return
	}
}
func childProcess() int {

	code, err := strconv.Atoi(os.Args[2])
	if err != nil {
		return 2
	}
	if code != 0 {
		return code
	}
	spts := os.Args[3]
	if spts == "" {
		return 3
	}
	println()
	evns := utils.AllEnv()
	//evns := os.Environ()
	bts, err := json.Marshal(evns)
	if err != nil {
		println("sys env err:" + err.Error())
		return 4
	}
	println(childprefix + spts)
	println(string(bts))
	time.Sleep(time.Millisecond * 50)
	return code
}

/*func reads(){
	bts:=make([]byte,1024)
	n,err:=os.Stdin.Read(bts)
	if n>0{
		println("reads:",string(bts))
	}
	if err!=nil{

	}
}*/
