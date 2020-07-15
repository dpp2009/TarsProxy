package main

import (
	"encoding/binary"
	"fmt"
	"github.com/TarsCloud/TarsGo/tars"
	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
	"github.com/TarsCloud/TarsGo/tars/protocol/res/requestf"
	"io/ioutil"
	"net"
	"net/http"
	"queryf"
)

var clientLocator string

func main() {
	//Init servant

	//Get Config File Object
	cfg := tars.GetServerConfig()

	ccg := tars.GetClientConfig()
	clientLocator = ccg.Locator

	//http  Tars.Tarsproxy.HttpObj
	mux := &tars.TarsHttpMux{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello tafgo"))
	})
	mux.HandleFunc("/tup", func(w http.ResponseWriter, r *http.Request) {
		handlePostTup(w,r)
	})
	tars.AddHttpServant(mux, cfg.App+"."+cfg.Server+".HttpObj") //Register http server

	//start tars
	tars.Run()
}

func handlePostTup(w http.ResponseWriter, r *http.Request)  {
	method := r.Method
	if method!="POST" {
		_,_ = w.Write([]byte(method))
		return
	}
	fmt.Println("tup method:",method)

	reqBuffer, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("tup ReadAll error",err)
	}

	//解码请求buffer，获取服务名&方法名
	reqPackage := requestf.RequestPacket{}
	is := codec.NewReader(reqBuffer[4:])
	err = reqPackage.ReadFrom(is)
	if err != nil {
		fmt.Println("tup ReadFrom",err)
	}
	SServantName := reqPackage.SServantName
	SFuncName := reqPackage.SFuncName
	fmt.Println("tup SServantName.SFuncName ",SServantName,SFuncName)

	//获取服务的ip&port
	comm := tars.NewCommunicator()
	obj := fmt.Sprintf(clientLocator)
	app := new(queryf.Queryf)
	comm.StringToProxy(obj, app)
	endpointF, err := app.FindObjectById(SServantName)
	if err != nil {
		fmt.Println(err)
	}
	locator:= fmt.Sprintf("%s:%d",endpointF[0].Host,endpointF[0].Port)
	fmt.Println("tup locator ",locator)

	//转发buffer给服务
	var tcpAddr *net.TCPAddr
	tcpAddr,_ = net.ResolveTCPAddr("tcp",locator)
	conn,err := net.DialTCP("tcp",nil,tcpAddr)
	if err!=nil {
		fmt.Println("Client connect error ! " + err.Error())
		return
	}
	defer conn.Close()

	fmt.Println(conn.LocalAddr().String() + " : Client connected!")
	conn.Write(reqBuffer)

	respBuffer := recv(conn)
	_,_ = w.Write(respBuffer)
}

func recv(conn net.Conn) []byte {
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
			_, status := ParsePackage(currBuffer)
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

//TarsRequest parse full tars request from package
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
