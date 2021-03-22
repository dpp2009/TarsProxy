package main

import (
	"Local"
	"bufio"
	"bytes"
	"configf"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/TarsCloud/TarsGo/tars"
	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
	"github.com/TarsCloud/TarsGo/tars/protocol/res/requestf"
	"github.com/TarsCloud/TarsGo/tars/util/tools"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var logLevel string
var queryObjUrl string
var localTcpProxy string
var httpTarsGateWayUrl string
var configObjPathPre = "ConfigObj"

//SET CGO_ENABLED=0
//SET GOOS=darwin
//SET GOARCH=amd64

//SET CGO_ENABLED=0
//SET GOOS=linux
//SET GOARCH=amd64

//rsrc.exe -manifest ico.manifest -o app.syso -ico app.ico
//go build
//shell\bulid.bat
//Tarsproxy.exe  --config=Tarsproxy.conf
func main() {
	if len(os.Args)==1 {
		PrintLogo()
		//todo  监测配置文件变化 =》校验完整性 =》重启

		cmd := exec.Command(os.Args[0], "--config=Tarsproxy.conf")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			panic(err)
		}

		_ = cmd.Start()
		reader := bufio.NewReader(stdout)
		//实时循环读取输出流中的一行内容
		for {
			line, err2 := reader.ReadString('\n')
			if err2 != nil || io.EOF == err2 {
				break
			}
			fmt.Print(line)
		}
		_ = cmd.Wait()
		return
	}
	runTars()
}

func runTars()  {
	//Init servant
	//Get Config File Object
	cfg := tars.GetServerConfig()
	logLevel = cfg.LogLevel

	registryUrl := fmt.Sprintf("@tcp -h %s -p %d",cfg.Adapters["Local.Tarsproxy.RegistryObjObjAdapter"].Endpoint.Host,cfg.Adapters["Local.Tarsproxy.RegistryObjObjAdapter"].Endpoint.Port)
	fmt.Println("locator=taf.tafregistry.QueryObj"+registryUrl)
	fmt.Println("locator=tars.tarsregistry.QueryObj"+registryUrl)

	queryObjUrl = fmt.Sprintf("%s:%d",
		cfg.Adapters["Local.Tarsproxy.RegistryObjObjAdapter"].Endpoint.Host,
		cfg.Adapters["Local.Tarsproxy.RegistryObjObjAdapter"].Endpoint.Port)
	localTcpProxy = fmt.Sprintf("%s:%d",
		cfg.Adapters["Local.Tarsproxy.localTcpProxy"].Endpoint.Host,
		cfg.Adapters["Local.Tarsproxy.localTcpProxy"].Endpoint.Port)
	httpTarsGateWayUrl = getConfUrl(
		cfg.Adapters["Local.Tarsproxy.httpTarsGateWay"].Endpoint.Host,
		cfg.Adapters["Local.Tarsproxy.httpTarsGateWay"].Endpoint.Port,
		cfg.Adapters["Local.Tarsproxy.httpTarsGateWay"].Protocol)

	//tars  Local.Tarsproxy.RegistryObj
	imp := new(RegistryObjImp)
	app := new(Local.RegistryObj)
	app.AddServant(imp, cfg.App+"."+cfg.Server+".RegistryObj")

	//http  Local.Tarsproxy.HttpObj
	mux := &tars.TarsHttpMux{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_,_ = w.Write([]byte("Hello Tars, UI TODO"))
	})
	mux.HandleFunc("/FindObjectById", func(w http.ResponseWriter, r *http.Request) {
		endpointF,_ :=imp.FindObjectById("")
		data, _ := json.Marshal(endpointF)
		_,_ = w.Write([]byte(data))
	})
	mux.HandleFunc("/kill", func(w http.ResponseWriter, r *http.Request) {
		_,_ = w.Write([]byte("kill ok"))
	})
	tars.AddHttpServant(mux, cfg.App+"."+cfg.Server+".HttpObj") //Register http server

	if isDebug() {
		url := fmt.Sprintf("http://%s:%d", cfg.Adapters["Local.Tarsproxy.HttpObjAdapter"].Endpoint.Host, cfg.Adapters["Local.Tarsproxy.HttpObjAdapter"].Endpoint.Port)
		go OpenUrl(url)
	}

	//start proxy
	go proxy()

	//start tars
	tars.Run()
}

func getConfUrl(host string,port int32,path string) string {
	if( port==443 ){
		return fmt.Sprintf("https://%s:%d%s", host, port, path)
	}
	return fmt.Sprintf("http://%s:%d%s", host, port, path)
}

//tcp <=> httpPost
func proxy() {
	Infolog("")
	Infolog("LocalTcpProxy " + localTcpProxy)
	Infolog("HttpTarsGateWayUrl " + httpTarsGateWayUrl)

	fromlistener, err := net.Listen("tcp", localTcpProxy)

	if err != nil {
		Errorlog("Unable to listen on: %s, error: %s\n", localTcpProxy, err.Error())
	}
	defer fromlistener.Close()

	for {
		fromcon, err := fromlistener.Accept()
		if err != nil {
			Errorlog("proxy Unable to accept a request, error: %s ", err.Error())
		} else {
			Debuglog("proxy new connect:" + fromcon.RemoteAddr().String())
		}

		go recv(fromcon)
	}
}

func recv(conn net.Conn) {
	buffer := make([]byte, 1024*4)
	var currBuffer []byte
	var n int
	var err error
	var n2 int
	for {
		n, err = conn.Read(buffer)
		n2 += n
		if err != nil {
			return
		}
		currBuffer = append(currBuffer, buffer[:n]...)
		//fmt.Printf("recv n:%d n2:%d bufferLen:%d currBufferLen:%d \n",n,n2,len(buffer),len(currBuffer))
		for {
			pkgLen, status := parsePackage(currBuffer)
			if status == PACKAGE_LESS {
				break
			}
			if status == PACKAGE_FULL {
				//fmt.Printf("recv pkgLen:%d bufferLen:%d currBufferLen:%d  \n",pkgLen, len(buffer),len(currBuffer))

				postBuffer := httpPost(currBuffer);
				_,_ = conn.Write(postBuffer)

				currBuffer = currBuffer[pkgLen:]
				if len(currBuffer) > 0 {
					continue
				}
				currBuffer = nil
				break
			}
			Errorlog("parse package error")
			return
		}
	}
}

func httpPost(buffer []byte)([]byte) {
	//taf.tafconfig.ConfigObj loadConfigByInfo
	//tars.tarsconfig.ConfigObj

	startTime := time.Now().UnixNano()
	var body []byte

	reqPackage := requestf.RequestPacket{}
	if len(buffer)<4 {
		Debuglog("httpPost bufferlen",len(buffer))
	}else {

		//fmt.Println( string(buffer) )
		is := codec.NewReader(buffer[4:])
		err := reqPackage.ReadFrom(is)
		if err != nil {
			fmt.Println(err)
		}
		Infolog("%s:%s IVersion:%d len:%d ", reqPackage.SServantName,reqPackage.SFuncName,reqPackage.IVersion, len(buffer))

		if IsConfigObj(reqPackage) {
			body = getConfFile(reqPackage)
		}
		if IsQueryObj(reqPackage) {
			body = _TcpSend(queryObjUrl,buffer)
		}
		if IsStatObj(reqPackage) || IsPropertyObj(reqPackage) || IsNodeServerObj(reqPackage) {
			body = ReturnIntZeroBodyBuffer(reqPackage)
		}
	}

	//body = _httpPost(buffer)
	//fmt.Println("_httpPost getConfFile bodylen ",len(body))

	if len(body)>0 {
		Debuglog("httpPost useData bodylen",len(body))
	}else {
		body = _httpPost(buffer)
	}

	responsePacket := requestf.ResponsePacket{}
	if len(body)<4 {
		Debuglog("httpPost bodylen",len(body))
	}else {
		//fmt.Println( string(body) )
		is := codec.NewReader(body[4:])
		_ = responsePacket.ReadFrom(is)
		Debuglog("IRet:%d len:%d SResultDesc:%s ",responsePacket.IRet, len(body),responsePacket.SResultDesc)

		if IsConfigObj(reqPackage) {
			go saveConfFile(reqPackage,responsePacket)
		}
	}

	endTime := time.Now().UnixNano()
	milliSeconds:= float64((endTime - startTime) / 1e6)
	if milliSeconds>10 {
		Infolog("%s:%s milliSeconds:%d ", reqPackage.SServantName,reqPackage.SFuncName,int(milliSeconds))
	}
	return body
}

//本地没有  保存到本地
func saveConfFile(reqPackage requestf.RequestPacket,responsePacket requestf.ResponsePacket) {
	if IsConfigObj(reqPackage) {
		var appServerName string
		var filePath string
		var filename string
		var config string

		_is := codec.NewReader(tools.Int8ToByte(reqPackage.SBuffer))
		_os := codec.NewReader(tools.Int8ToByte(responsePacket.SBuffer))

		if reqPackage.SFuncName=="loadConfig" {
			var app string
			var server string
			_ = _is.Read_string(&app, 1, true)
			_ = _is.Read_string(&server, 2, true)
			appServerName = app +"."+ server
			_ = _is.Read_string(&filename, 3, true)
			_ = _is.Read_string(&filename, 3, true)
			filePath = fmt.Sprintf("%s/%s/%s", configObjPathPre,appServerName,filename)
			_ = _os.Read_string(&config, 4, true)

		}else if reqPackage.SFuncName=="loadConfigByHost" {
			var host string
			_ = _is.Read_string(&appServerName, 1, true)
			_ = _is.Read_string(&filename, 2, true)
			_ = _is.Read_string(&host, 3, true)
			if host=="" {
				filePath = fmt.Sprintf("%s/%s/%s", configObjPathPre,appServerName,filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s", configObjPathPre,appServerName,host,filename)
			}
			_ = _os.Read_string(&config, 4, true)

		}else if reqPackage.SFuncName=="loadConfigByInfo" {
			var configInfo configf.ConfigInfo
			_ = configInfo.ReadBlock(_is, 1, true)
			appServerName = configInfo.Appname +"."+ configInfo.Servername
			filename = configInfo.Filename
			if configInfo.Host=="" {
				filePath = fmt.Sprintf("%s/%s/%s", configObjPathPre,appServerName,filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s", configObjPathPre,appServerName,configInfo.Host,filename)
			}
			_ = _os.Read_string(&config, 2, true)
		}

		if !CheckFileIsExist(filePath) && config!="" {
			path := strings.Replace(filePath,"/"+filename,"",1)
			fmt.Println("saveConfFile MkdirAll path:", path)
			_ = os.MkdirAll(path, os.ModePerm);
			file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0766)
			if err != nil {
				fmt.Println("open file error:", err)
				return
			}
			//写文件，输出到文件
			_,_ = fmt.Fprint(file, config)
			defer file.Close()

			Infolog("saveConfFile filePath %s ", filePath)
		}
	}
}

//本地有 直接读取本地
func getConfFile(reqPackage requestf.RequestPacket) []byte {
	var body []byte
	if IsConfigObj(reqPackage) {
		var filePath string
		_is := codec.NewReader(tools.Int8ToByte(reqPackage.SBuffer))
		_os := codec.NewBuffer()
		var _funRet_ int32
		var appServerName string

		if reqPackage.SFuncName=="loadConfig" {
			var app string
			var server string
			_ = _is.Read_string(&app, 1, true)
			_ = _is.Read_string(&server, 2, true)
			appServerName = app +"."+ server

			var filename string
			_ = _is.Read_string(&filename, 3, true)
			filePath = fmt.Sprintf("%s/%s/%s", configObjPathPre,appServerName,filename)

			if CheckFileIsExist(filePath) {
				data, err := ioutil.ReadFile(filePath)
				if err != nil {
					fmt.Println("read file err:", err.Error())
				}else {
					_os.Reset()
					_ = _os.Write_int32(_funRet_, 0)
					_ = _os.Write_string(string(data), 4)
				}
				fmt.Println("getConfFile filePath ", filePath)
			}
		}else if reqPackage.SFuncName=="loadConfigByHost" {
			_ = _is.Read_string(&appServerName, 1, true)

			var filename string
			var host string
			_ = _is.Read_string(&filename, 2, true)
			_ = _is.Read_string(&host, 3, true)
			if host=="" {
				filePath = fmt.Sprintf("%s/%s/%s", configObjPathPre,appServerName,filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s", configObjPathPre,appServerName,host,filename)
			}

			if CheckFileIsExist(filePath) {
				data, err := ioutil.ReadFile(filePath)
				if err != nil {
					fmt.Println("read file err:", err.Error())
				}else {
					_os.Reset()
					err = _os.Write_int32(_funRet_, 0)
					err = _os.Write_string(string(data), 4)
				}
				fmt.Println("getConfFile filePath ", filePath)
			}
		}else if reqPackage.SFuncName=="loadConfigByInfo" {
			var configInfo configf.ConfigInfo
			_ = configInfo.ReadBlock(_is, 1, true)
			appServerName = configInfo.Appname +"."+ configInfo.Servername
			if configInfo.Host=="" {
				filePath = fmt.Sprintf("%s/%s/%s", configObjPathPre,appServerName,configInfo.Filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s", configObjPathPre,appServerName,configInfo.Host,configInfo.Filename)
			}

			//fmt.Println("xxxxx loadConfigByInfo filePath:", filePath)
			if CheckFileIsExist(filePath) {
				data, err := ioutil.ReadFile(filePath)
				if err != nil {
					fmt.Println("read file err:", err.Error())
				}else {
					_os.Reset()
					err = _os.Write_int32(_funRet_, 0)
					err = _os.Write_string(string(data), 2)
				}
				fmt.Println("getConfFile filePath ", filePath)
			}
		}

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
	}
	return body
}


func _httpPost(buffer []byte)([]byte) {
	resp, err := http.Post(httpTarsGateWayUrl,"",bytes.NewBuffer(buffer))
	if err != nil {
		Errorlog("tarsGateWay httpPost error: %s",err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Errorlog("tarsGateWay httpPost error: %s",err)
	}
	return body
}

func getReturnBuffer(rspPackage *requestf.ResponsePacket) []byte {
	_os := codec.NewBuffer()
	_ = rspPackage.WriteTo(_os)
	bs := _os.ToBytes()
	sbuf := bytes.NewBuffer(nil)
	sbuf.Write(make([]byte, 4))
	sbuf.Write(bs)
	length := sbuf.Len()
	binary.BigEndian.PutUint32(sbuf.Bytes(), uint32(length))
	re := sbuf.Bytes()
	return re
}

const (
	iMaxLength = 10485760
)
const (
	PACKAGE_LESS = iota
	PACKAGE_FULL
	PACKAGE_ERROR
)

func parsePackage(buff []byte) (int, int) {
	return tarsRequest(buff)
}

//TarsRequest parse full tars request from package
func tarsRequest(rev []byte) (int, int) {
	if len(rev) < 4 {
		return 0, PACKAGE_LESS
	}
	iHeaderLen := int(binary.BigEndian.Uint32(rev[0:4]))
	if iHeaderLen < 4 || iHeaderLen > iMaxLength {
		return 0, PACKAGE_ERROR
	}
	if len(rev) < iHeaderLen {
		return 0, PACKAGE_LESS
	}
	return iHeaderLen, PACKAGE_FULL
}

func _TcpSend(locator string,reqBuffer []byte)([]byte) {
	Debuglog("_TcpSend locator ",locator)

	//转发buffer给服务
	var tcpAddr *net.TCPAddr
	tcpAddr,_ = net.ResolveTCPAddr("tcp",locator)
	conn,err := net.DialTCP("tcp",nil,tcpAddr)
	defer conn.Close()
	if err!=nil {
		Errorlog("_TcpSend Client connect error: %s " + err.Error())
		return nil
	}

	Debuglog("_TcpSend "+conn.LocalAddr().String() + " : Client connected!")
	_,_ = conn.Write(reqBuffer)

	respBuffer := _TcpSendRecv(conn)
	return respBuffer
}

func _TcpSendRecv(conn net.Conn) []byte {
	buffer := make([]byte, 1024*4)
	var currBuffer []byte
	var n int
	var err error
	var n2 int
	for {
		n, err = conn.Read(buffer)
		n2 += n
		if err != nil {
			return nil
		}
		currBuffer = append(currBuffer, buffer[:n]...)
		//fmt.Printf("recv n:%d n2:%d bufferLen:%d currBufferLen:%d \n",n,n2,len(buffer),len(currBuffer))
		for {
			_, status := parsePackage(currBuffer)
			if status == PACKAGE_LESS {
				break
			}
			if status == PACKAGE_FULL {
				//fmt.Printf("recv pkgLen:%d bufferLen:%d currBufferLen:%d  \n",pkgLen, len(buffer),len(currBuffer))
				return currBuffer
			}
			fmt.Printf("parse package error")
			return nil
		}
	}
}

//func handleRequestBuffer(req []byte)  {
//	//fmt.Println( string(req) )
//	if len(req)<4 {
//		Errorlog("handleRequestBuffer len",len(req))
//		return
//	}
//	reqPackage := requestf.RequestPacket{}
//	is := codec.NewReader(req[4:])
//	err := reqPackage.ReadFrom(is)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	fmt.Printf("%s:%s IVersion:%d len:%d \n", reqPackage.SServantName,reqPackage.SFuncName,reqPackage.IVersion, len(req))
//}
//
//func handleResponseBuffer(req []byte)  {
//	if len(req)<4 {
//		Errorlog("handleResponseBuffer len",len(req))
//		return
//	}
//	fmt.Println( string(req) )
//	responsePacket := requestf.ResponsePacket{}
//	is := codec.NewReader(req[4:])
//	responsePacket.ReadFrom(is)
//
//	Debuglog("IRet:%d len:%d SResultDesc:%s ",responsePacket.IRet, len(req),responsePacket.SResultDesc)
//}

