package main

import (
	"Local"
	"PHPTest"
	"fmt"
	"github.com/TarsCloud/TarsGo/tars"
)

func main() {

	//test1()
	//test2()
	//test3()
	//test4()

}

func test4()  {
	//test proxy

	comm := tars.NewCommunicator()
	obj := fmt.Sprintf("PHPTest.PHPServer.obj@tcp -h 127.0.0.1 -p 10017 -t 60000")
	app := new(PHPTest.PHPServer)
	comm.StringToProxy(obj, app)

	app.TestTafServer()
}

func test3()  {
	//test proxy

	comm := tars.NewCommunicator()
	obj := fmt.Sprintf("PHPTest.PHPServer.obj@tcp -h 127.0.0.1 -p 10017 -t 60000")
	app := new(PHPTest.PHPServer)
	comm.StringToProxy(obj, app)

	var D bool
	var E int32
	var F string

	for i := 0; i < 10; i++ {
		ret,err := app.TestBasic(false,2,"3",&D,&E,&F);
		fmt.Println(ret,D,E,F)
		if err != nil {
			fmt.Println("test3 err",err)
		}
		//time.Sleep(1* time.Second)
	}
}

func test2()  {
	//test proxy

	comm := tars.NewCommunicator()
	obj := fmt.Sprintf("PHPTest.PHPServer.obj@tcp -h 127.0.0.1 -p 10017 -t 60000")
	app := new(PHPTest.PHPServer)
	comm.StringToProxy(obj, app)

	var outGreetings string

	value := "1111xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	for i:=0; i<10; i++ {
		value += "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	}
	value += "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx333"

	value = "aaaaa"
	err := app.SayHelloWorld(value,&outGreetings);
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(outGreetings)
}

func test1()  {
	comm := tars.NewCommunicator()
	obj := fmt.Sprintf("Local.Tarsproxy.RegistryObj@tcp -h 127.0.0.1 -p 10015 -t 60000")
	app := new(Local.RegistryObj)
	comm.StringToProxy(obj, app)

	//var out, i int32
	//i = 123
	//ret, err := app.Add(i, i*2, &out)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Println(ret, out)

	endpointF, err := app.FindObjectById("App.Server.Obj");
	println(endpointF[0].Host,endpointF[0].Port)
	if err != nil {
		fmt.Println(err)
		return
	}

	var activeEp []Local.EndpointF
	var inactiveEp []Local.EndpointF
	ret,err := app.FindObjectByIdInSameGroup("App.Server.Obj",&activeEp,&inactiveEp);
	if err != nil {
		fmt.Println(err)
		return
	}
	println(ret,activeEp[0].Host,activeEp[0].Port)
}
