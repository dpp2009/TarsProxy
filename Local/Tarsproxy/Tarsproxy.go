package main

import (
	"Local"
	"bytes"
	"configf"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/TarsCloud/TarsGo/tars"
	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
	"github.com/TarsCloud/TarsGo/tars/protocol/res/requestf"
	"github.com/TarsCloud/TarsGo/tars/util/tools"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var localTcpProxy string
var httpTarsGateWayUrl string
var clientLocator string
var ConfigObjPathPre = "ConfigObj/conf"

//Tarsproxy.exe  --config=Tarsproxy.conf
func main() {
	//Init servant

	//Get Config File Object
	cfg := tars.GetServerConfig()
	localTcpProxy = fmt.Sprintf("%s:%d",
		cfg.Adapters["Local.Tarsproxy.localTcpProxy"].Endpoint.Host,
		cfg.Adapters["Local.Tarsproxy.localTcpProxy"].Endpoint.Port)
	httpTarsGateWayUrl = fmt.Sprintf("http://%s:%d%s",
		cfg.Adapters["Local.Tarsproxy.httpTarsGateWay"].Endpoint.Host,
		cfg.Adapters["Local.Tarsproxy.httpTarsGateWay"].Endpoint.Port,
		cfg.Adapters["Local.Tarsproxy.httpTarsGateWay"].Protocol)

	ccg := tars.GetClientConfig()
	clientLocator = ccg.Locator

	//tars  Local.Tarsproxy.RegistryObj
	imp := new(RegistryObjImp)
	app := new(Local.RegistryObj)
	app.AddServant(imp, cfg.App+"."+cfg.Server+".RegistryObj")

	//http  Local.Tarsproxy.HttpObj
	mux := &tars.TarsHttpMux{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello tafgo"))
	})
	mux.HandleFunc("/FindObjectById", func(w http.ResponseWriter, r *http.Request) {
		endpointF,_ :=imp.FindObjectById("")
		data, _ := json.Marshal(endpointF)
		w.Write([]byte(data))
	})
	tars.AddHttpServant(mux, cfg.App+"."+cfg.Server+".HttpObj") //Register http server

	//start proxy
	go proxy()

	//start tars
	tars.Run()
}

func getReturnBuffer(rspPackage *requestf.ResponsePacket) []byte {
	os := codec.NewBuffer()
	rspPackage.WriteTo(os)
	bs := os.ToBytes()
	sbuf := bytes.NewBuffer(nil)
	sbuf.Write(make([]byte, 4))
	sbuf.Write(bs)
	len := sbuf.Len()
	binary.BigEndian.PutUint32(sbuf.Bytes(), uint32(len))
	re := sbuf.Bytes()
	return re
}

//tcp <=> httpPost
func proxy() {

	fmt.Println("localTcpProxy " + localTcpProxy)
	fmt.Println("httpTarsGateWayUrl " + httpTarsGateWayUrl)

	fromlistener, err := net.Listen("tcp", localTcpProxy)

	if err != nil {
		log.Fatal("Unable to listen on: %s, error: %s\n", localTcpProxy, err.Error())
	}
	defer fromlistener.Close()

	for {
		fromcon, err := fromlistener.Accept()
		if err != nil {
			fmt.Printf("proxy Unable to accept a request, error: %s\n", err.Error())
		} else {
			fmt.Println("proxy new connect:" + fromcon.RemoteAddr().String())
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
			pkgLen, status := ParsePackage(currBuffer)
			if status == PACKAGE_LESS {
				break
			}
			if status == PACKAGE_FULL {
				//fmt.Printf("recv pkgLen:%d bufferLen:%d currBufferLen:%d  \n",pkgLen, len(buffer),len(currBuffer))

				postBuffer := httpPost(currBuffer);
				conn.Write(postBuffer)

				currBuffer = currBuffer[pkgLen:]
				if len(currBuffer) > 0 {
					continue
				}
				currBuffer = nil
				break
			}
			fmt.Printf("parse package error")
			return
		}
	}
}

func httpPost(buffer []byte)([]byte) {
	//taf.tafconfig.ConfigObj loadConfigByInfo
	//tars.tarsconfig.ConfigObj

	var body []byte

	reqPackage := requestf.RequestPacket{}
	if len(buffer)<4 {
		fmt.Println("httpPost bufferlen",len(buffer))
	}else {
		//fmt.Println( string(buffer) )
		is := codec.NewReader(buffer[4:])
		err := reqPackage.ReadFrom(is)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("%s:%s IVersion:%d len:%d \n", reqPackage.SServantName,reqPackage.SFuncName,reqPackage.IVersion, len(buffer))

		if isConfigObj(reqPackage) {
			body = getConfFile(reqPackage)
		}
	}

	if len(body)>0 {
		fmt.Println("httpPost useLocalConf bodylen",len(body))
	}else {
		body = _httpPost(buffer)
	}

	responsePacket := requestf.ResponsePacket{}
	if len(body)<4 {
		fmt.Println("httpPost bodylen",len(body))
	}else {
		//fmt.Println( string(body) )
		is := codec.NewReader(body[4:])
		responsePacket.ReadFrom(is)
		fmt.Printf("IRet:%d len:%d SResultDesc:%s \n",responsePacket.IRet, len(body),responsePacket.SResultDesc)

		if isConfigObj(reqPackage) {
			go saveConfFile(reqPackage,responsePacket)
		}
	}
	return body
}

//save file when local don't have
func saveConfFile(reqPackage requestf.RequestPacket,responsePacket requestf.ResponsePacket) {
	if isConfigObj(reqPackage) {
		var filePath string
		var filename string
		var config string

		_is := codec.NewReader(tools.Int8ToByte(reqPackage.SBuffer))
		_os := codec.NewReader(tools.Int8ToByte(responsePacket.SBuffer))

		if reqPackage.SFuncName=="loadConfig" {
			_ = _is.Read_string(&filename, 3, true)
			filePath = fmt.Sprintf("%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,filename)
			_ = _os.Read_string(&config, 4, true)

		}else if reqPackage.SFuncName=="loadConfigByHost" {
			var host string
			_ = _is.Read_string(&filename, 2, true)
			_ = _is.Read_string(&host, 3, true)
			if host=="" {
				filePath = fmt.Sprintf("%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,host,filename)
			}
			_ = _os.Read_string(&config, 4, true)

		}else if reqPackage.SFuncName=="loadConfigByInfo" {
			var configInfo configf.ConfigInfo
			_ = configInfo.ReadBlock(_is, 1, true)
			filename = configInfo.Filename
			if configInfo.Host=="" {
				filePath = fmt.Sprintf("%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,configInfo.Host,filename)
			}
			_ = _os.Read_string(&config, 2, true)
		}

		if !checkFileIsExist(filePath) && config!="" {
			path := strings.Replace(filePath,"/"+filename,"",1)
			fmt.Println("saveConfFile MkdirAll path:", path)
			_ = os.MkdirAll(path, os.ModePerm);
			_ = ioutil.WriteFile(filePath, []byte(config), 0666)
			fmt.Println("saveConfFile filePath ", filePath)
		}
	}
}

//read file conf from local first
func getConfFile(reqPackage requestf.RequestPacket) []byte {
	var body []byte
	if isConfigObj(reqPackage) {
		var filePath string
		_is := codec.NewReader(tools.Int8ToByte(reqPackage.SBuffer))
		_os := codec.NewBuffer()
		var _funRet_ int32

		if reqPackage.SFuncName=="loadConfig" {
			var filename string
			_ = _is.Read_string(&filename, 3, true)
			filePath = fmt.Sprintf("%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,filename)

			if checkFileIsExist(filePath) {
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
			var filename string
			var host string
			_ = _is.Read_string(&filename, 2, true)
			_ = _is.Read_string(&host, 3, true)
			if host=="" {
				filePath = fmt.Sprintf("%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,host,filename)
			}

			if checkFileIsExist(filePath) {
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
			if configInfo.Host=="" {
				filePath = fmt.Sprintf("%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,configInfo.Filename)
			}else {
				filePath = fmt.Sprintf("%s/%s/%s/%s",ConfigObjPathPre,reqPackage.SServantName,configInfo.Host,configInfo.Filename)
			}

			if checkFileIsExist(filePath) {
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

func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func isConfigObj(reqPackage requestf.RequestPacket) bool {
	if reqPackage.SServantName=="taf.tafconfig.ConfigObj" || reqPackage.SServantName=="tars.tarsconfig.ConfigObj" {
		if reqPackage.SFuncName=="loadConfig" || reqPackage.SFuncName=="loadConfigByHost" || reqPackage.SFuncName=="loadConfigByInfo" {
			return true;
		}
	}
	return false;
}

func _httpPost(buffer []byte)([]byte) {
	resp, err := http.Post(httpTarsGateWayUrl,"",bytes.NewBuffer(buffer))
	if err != nil {
		fmt.Println(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("tarsGateWay httpPost error",err)
	}
	return body
}

const (
	iMaxLength = 10485760
)
const (
	PACKAGE_LESS = iota
	PACKAGE_FULL
	PACKAGE_ERROR
)

func ParsePackage(buff []byte) (int, int) {
	return TarsRequest(buff)
}

func TarsRequest(rev []byte) (int, int) {
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
