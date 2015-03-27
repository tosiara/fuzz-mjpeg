package main

import (
	"bufio"
	"bytes"
	_ "encoding/json"
	"flag"
	"fmt"
	"github.com/skarademir/naturalsort"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FuzzedFrame struct {
	Boundary  string
	Filepath  string
	Framerate int
}
type FuzzedSession struct {
	ResponseHeader string
	FuzzedFrames   []FuzzedFrame
}

var ( // command line flag variables
	folderpath string
	boundary   string
	framerate  int
	hostname   string
	port       int
	fuzzmode   bool
)
var ( // fuzz command line flag variables
	fuzz_maxlength int64
	fuzz_datapath  string
	fuzz_savepath  string
)
var (
	fuzzedResponse   string
	fuzzedBoundaries []string
	bufrw            bufio.ReadWriter
)

func fuzzLength(length int) string {
	if rand.Intn(1) == 1 {
		return fmt.Sprintf("%d", length)
	} else {
		return fmt.Sprintf("%d", rand.Int63n(fuzz_maxlength))
	}
}
func fuzzBoundary() {
	boundary = "A" + strings.Repeat("A", rand.Intn(16384)) //TODO Investigate if a larger string is any use?
}
func fuzzFramerate() {
	framerate = rand.Intn(359) + 1 //TODO Add a way to increase framerate range from 1-1/360
}
func getFuzzFiles() {
	//get response files
	responseFiles, _ := filepath.Glob(fuzz_datapath + "/*.response.txt")
	for responseFile := range responseFiles {
		dat, err := ioutil.ReadFile(responseFiles[responseFile])
		if err != nil {
			panic(err)
		}
		fuzzedResponse = string(dat)
		break // can be changed to handle many files. we just pick the first one
	}
	//get boundary files
	boundaryFiles, _ := filepath.Glob(fuzz_datapath + "/*.boundary.txt")
	for boundaryFile := range boundaryFiles {
		dat, err := ioutil.ReadFile(boundaryFiles[boundaryFile])
		if err != nil {
			panic(err)
		}
		fuzzedBoundaries = append(fuzzedBoundaries, string(dat))
	}
}
func handler(w http.ResponseWriter, r *http.Request) {
	//set header to multipart and describe the boundary name to be used elsewhere
	if fuzzmode && len(fuzzedResponse) > 0 {
		fmt.Printf("what\n")
		data := &bytes.Buffer{}
		data.Write([]byte("Content-Type: multipart/x-mixed-replace;boundary=" + boundary + "\n" + fuzzedResponse))

		w.Header().Write(data)
		data.Write([]byte{'\r', '\n'})
		w.Header().Set("Content-Type", "multipart/x-mixed-replace;boundary="+boundary) //"multipart/x-mixed-replace;boundary=<boundary-name>")

		fmt.Printf("%#f\n", w.Header().Get(""))
	} else {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace;boundary="+boundary) //"multipart/x-mixed-replace;boundary=<boundary-name>")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")
	}
	//load file(s) from folderpath
	files, _ := filepath.Glob(folderpath + "/*.jpeg")
	sort.Sort(naturalsort.NaturalSort(files))
	for file := range files {
		dat, err := ioutil.ReadFile(files[file])
		if err != nil {
			panic(err)
		}
		w.Write([]byte("\n--" + boundary + "\n"))

		var length string = fmt.Sprintf("%d", len(dat))
		if fuzzmode {
			fuzzFramerate()
			if len(fuzzedBoundaries) > 0 {
				w.Write([]byte(fuzzedBoundaries[file%len(fuzzedBoundaries)]))
			} else {
				w.Write([]byte("Content-Type: image/jpeg\nContent-length: " + fuzzLength(len(dat))))
			}

		} else {
			w.Write([]byte("Content-Type: image/jpeg\nContent-length: " + length))
		}

		w.Write([]byte("\r\n\r\n"))
		w.Write(dat)

		time.Sleep(time.Second / time.Duration(framerate))
	}
}
func init() {
	//define command line flags
	flag.StringVar(&folderpath, "folderpath", "./1.mjpeg", "Location of jpeg files to be sent (in natural ascending order) to clients. Default: ./1.mjpeg/")
	flag.StringVar(&boundary, "boundary", "<boundary-name>", "Name of the boundary used between frames. Default: '<boundary-name>'")
	flag.IntVar(&framerate, "framerate", 10, "Framerate in frames per second. Default: 10")
	flag.StringVar(&hostname, "hostname", "localhost", "Hostname. Default: 'localhost'")
	flag.IntVar(&port, "port", 8080, "Serving port Default: 8080")
	flag.BoolVar(&fuzzmode, "fuzzmode", false, "Fuzzing Switch. If this is set, All params are ignored (except folderpath) Default: false")
	//define fuzzer command line flags
	flag.Int64Var(&fuzz_maxlength, "fuzz_maxlength", math.MaxInt64, "Fuzzer Only: maximum reported frame length")
	flag.StringVar(&fuzz_datapath, "fuzz_datapath", "./1.mjpeg", "Location of fuzzed response.txt and boundary.txt files to be sent to clients. Default: ./1.mjpeg/")
	flag.StringVar(&fuzz_savepath, "fuzz_savepath", "./", "Location of saved session json files. Default: current directory")
	rand.Seed(42)
}
func main() {
	flag.Parse()
	if fuzzmode {
		fuzzBoundary()
		getFuzzFiles()
	}
	http.HandleFunc("/", handler)
	if port > 65535 || port < 0 {
		fmt.Printf("bad port")
		return
	}
	fulladdr := net.JoinHostPort(hostname, strconv.Itoa(port))
	log.Fatal(http.ListenAndServe(fulladdr, nil))
}
