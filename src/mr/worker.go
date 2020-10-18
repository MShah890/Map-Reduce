package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"strconv"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	args, reply := GetTaskArgs{}, GetTaskReply{}
	count := 0
	for true {
		call("Master.GetTask", &args, &reply)

		if reply.TaskType == "Map" {
			fmt.Println(count)
			count++
			doMap(reply.FilePath, mapf, reply.MapTaskNum, reply.ReduceTaskCount)
		}
	}

}

func doMap(filePath string, mapf func(string, string) []KeyValue, mapTaskNum int, reduceTaskCount int) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("cannot open %v", filePath)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filePath)
	}
	file.Close()
	kva := mapf(filePath, string(content))
	fmt.Println("Returned from map")
	kvalListPerRed := doPartition(kva, reduceTaskCount)
	fmt.Println("Returned From Partition")
	fileNames := make([]string, reduceTaskCount)
	for i := 0; i < reduceTaskCount; i++ {
		fileNames[i] = WriteToJSONFile(kvalListPerRed[i], mapTaskNum, i)
	}
	fmt.Println("Returned From WritingJSON")

	cargs, creply := CompleteTaskArgs{}, CompleteTaskReply{}
	cargs.TaskType = "Map"
	cargs.FilePathList = fileNames
	cargs.FileSplitName = filePath
	call("Master.CompleteTask", &cargs, &creply)
}

func doPartition(kva []KeyValue, reduceTaskCount int) [][]KeyValue {
	kvalListPerRed := make([][]KeyValue, reduceTaskCount)
	for _, kv := range kva {
		v := ihash(kv.Key) % reduceTaskCount
		kvalListPerRed[v] = append(kvalListPerRed[v], kv)
	}
	return kvalListPerRed
}

// WriteToJSONFile writes key value pairs to a json file for a particular reduce task
func WriteToJSONFile(intermediate []KeyValue, mapTaskNum, reduceTaskNUm int) string {
	fmt.Println("Writing file for task " + strconv.Itoa(mapTaskNum))
	filename := "temp-mr-" + strconv.Itoa(mapTaskNum) + "-" + strconv.Itoa(reduceTaskNUm)
	jfile, _ := os.Create(filename)

	enc := json.NewEncoder(jfile)
	for _, kv := range intermediate {
		err := enc.Encode(&kv)
		if err != nil {
			log.Fatal("error: ", err)
		}
	}
	return filename
}

//
// example function to show how to make an RPC call to the master.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	call("Master.Example", &args, &reply)

	// reply.Y should be 100.
	fmt.Printf("reply.Y %v\n", reply.Y)
}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
