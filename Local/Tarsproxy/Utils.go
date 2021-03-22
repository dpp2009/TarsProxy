package main

import (
	"fmt"
	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
	"github.com/TarsCloud/TarsGo/tars/protocol/res/requestf"
	"github.com/TarsCloud/TarsGo/tars/util/tools"
	"os"
	"os/exec"
	"runtime"
	"time"
)

func isDebug() bool {
	if logLevel=="debug" {
		return true;
	}
	return false;
}

func Debuglog(format string, a ...interface{}) {
	if isDebug() {
		fmt.Printf(format+"\r\n", a...)
	}
}

func Infolog(format string, a ...interface{}) {
	fmt.Printf(format+"\r\n", a...)
}

func Errorlog(format string, a ...interface{}) {
	fmt.Printf("ERROR "+format+"\r\n", a...)
}

//int reportMicMsg( map<StatMicMsgHead,StatMicMsgBody> msg, bool bFromClient);
//int reportSampleMsg(vector<StatSampleMsg> msg);
func ReturnIntZeroBodyBuffer(reqPackage requestf.RequestPacket) []byte {
	var body []byte

	_os := codec.NewBuffer()
	_ = _os.Write_int32(0, 0)

	if len(_os.ToBytes())>0 {
		var _status map[string]string
		var _context map[string]string
		tarsResp := requestf.ResponsePacket{
			IVersion:     reqPackage.IVersion,
			CPacketType:  reqPackage.CPacketType,
			IRequestId:   reqPackage.IRequestId,
			IMessageType: reqPackage.IMessageType,
			IRet:         0,
			SBuffer:      tools.ByteToInt8(_os.ToBytes()),
			Status:       _status,
			SResultDesc:  "",
			Context:      _context,
		}
		body = getReturnBuffer(&tarsResp)
	}
	return body
}

func IsConfigObj(reqPackage requestf.RequestPacket) bool {
	if reqPackage.SServantName=="taf.tafconfig.ConfigObj" || reqPackage.SServantName=="tars.tarsconfig.ConfigObj" {
		if reqPackage.SFuncName=="loadConfig" || reqPackage.SFuncName=="loadConfigByHost" || reqPackage.SFuncName=="loadConfigByInfo" {
			return true;
		}
	}
	return false;
}

func IsQueryObj(reqPackage requestf.RequestPacket) bool {
	if reqPackage.SServantName=="taf.tafregistry.QueryObj" || reqPackage.SServantName=="tars.tarsregistry.QueryObj" {
		return true;
	}
	return false;
}

func IsStatObj(reqPackage requestf.RequestPacket) bool {
	if reqPackage.SServantName=="taf.tafstat.StatObj" || reqPackage.SServantName=="tars.tarsstat.StatObj" {
		return true;
	}
	return false;
}

func IsPropertyObj(reqPackage requestf.RequestPacket) bool {
	if reqPackage.SServantName=="taf.tafproperty.PropertyObj" || reqPackage.SServantName=="tars.tarsproperty.PropertyObj" {
		return true;
	}
	return false;
}

func IsNodeServerObj(reqPackage requestf.RequestPacket) bool {
	if reqPackage.SServantName=="taf.tafnode.ServerObj" || reqPackage.SServantName=="tars.tarsnode.ServerObj" {
		return true;
	}
	return false;
}

func OpenUrl(uri string) {
	time.Sleep(time.Second * 3)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd","/c","start", uri)
	case "darwin":
		cmd = exec.Command("open", uri)
	case "linux":
		cmd = exec.Command("xdg-open", uri)
	default:
		return
	}
	Infolog("OpenUrl:"+uri)
	err := cmd.Start()
	if err != nil {
		Errorlog("OpenUrl: %s"+err.Error())
	}
}

/**
 * 判断文件是否存在  存在返回 true 不存在返回false
 */
func CheckFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func PrintLogo(){
	logo := `
 ___________   __        _______    ________  _______    ______  ___  ___  ___  ___  
("     _   ") /""\      /"      \  /"       )|   __ "\  /    " \|"  \/"  ||"  \/"  | 
 )__/  \\__/ /    \    |:        |(:   \___/ (. |__) :)// ____  \\   \  /  \   \  /  
    \\_ /   /' /\  \   |_____/   ) \___  \   |:  ____//  /    ) :)\\  \/    \\  \/   
    |.  |  //  __'  \   //      /   __/  \\  (|  /   (: (____/ // /\.  \    /   /    
    \:  | /   /  \\  \ |:  __   \  /" \   :)/|__/ \   \        / /  \   \  /   /     
     \__|(___/    \___)|__|  \___)(_______/(_______)   \"_____/ |___/\___||___/
`
	Infolog(logo)
	Infolog("https://github.com/dpp2009/TarsProxy")
	Infolog("")
}