package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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

var Session FuzzedSession

type FuzzedFrame struct {
	BoundaryHeader string
	Filepath       string
	Framerate      int
}
type FuzzedSession struct {
	Boundary       string
	ResponseHeader string
	FuzzedFrames   []FuzzedFrame
}

var ( // command line flag variables
	folderpath   string
	boundary     string
	framerate    int
	hostname     string
	port         int
	sessionpath  string
	generatemode bool
	playmode     bool
	fuzzmode     bool
)
var ( // generate command line flag variables
	generate_count int
)
var ( // fuzz command line flag variables
	fuzz_maxlength         int64
	fuzz_datapath          string
	fuzz_chance_badboundry int
)
var (
	fuzzedResponse   string
	fuzzedBoundaries []string

	bufrw bufio.ReadWriter
)

func fuzzLength(length int) string {
	if rand.Intn(1) == 1 {
		return fmt.Sprintf("%d", length)
	} else {
		return fmt.Sprintf("%d", rand.Int63n(fuzz_maxlength))
	}
}
func fuzzBoundary() {
	if rand.Intn(99) < (fuzz_chance_badboundry-1)%100 && len(fuzzedResponse) > 0 {
		boundary = fuzzedResponse
	} else {
		boundary = "A" + strings.Repeat("A", rand.Intn(16384)) //TODO Investigate if a larger string is any use?
	}
}
func fuzzFramerate() {
	framerate = rand.Intn(359*60) + 1 //TODO Add a way to increase framerate range from 1-1/360
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
func saveSession(Session *FuzzedSession) {
	data, _ := json.Marshal(Session)
	fmt.Printf("%#s", data)
	err := ioutil.WriteFile(sessionpath+"session."+strconv.FormatInt(time.Now().UnixNano(), 10)+".json", data, 0644)
	if err != nil {
		panic(err)
	}
}
func createSession(Session *FuzzedSession) {
	if fuzzmode {
		getFuzzFiles()
		fuzzBoundary()
	}
	Session.Boundary = boundary
	//set header to multipart and describe the boundary name to be used elsewhere
	var responseHeader string
	if fuzzmode && len(fuzzedResponse) > 0 {
		responseHeader = "Content-Type: multipart/x-mixed-replace;boundary=" + boundary + "\n" + fuzzedResponse
	} else {
		responseHeader = "Content-Type: multipart/x-mixed-replace;boundary=" + boundary + "\n" + "Connection: keep-alive" + "\n" + "Transfer-Encoding: chunked"
	}
	Session.ResponseHeader = responseHeader

	//load file(s) from folderpath
	files, _ := filepath.Glob(folderpath + "/*.jpeg")
	sort.Sort(naturalsort.NaturalSort(files))

	for file := range files {
		dat, err := ioutil.ReadFile(files[file])
		if err != nil {
			panic(err)
		}
		var Frame FuzzedFrame
		Frame.Filepath = files[file]

		var boundaryHeader string
		// set boundary header and framerate
		if fuzzmode {
			fuzzFramerate()                // fuzz the global framerate
			if len(fuzzedBoundaries) > 0 { //if there are premade fuzzed boundaries, use them
				boundaryHeader = fuzzedBoundaries[file%len(fuzzedBoundaries)]
			} else { // otherwise use the prepackaged one with a fake length
				boundaryHeader = "Content-Type: image/jpeg\nContent-length: " + fuzzLength(len(dat))
			}
		} else {
			//tell the truth
			var length string = fmt.Sprintf("%d", len(dat))
			boundaryHeader = "Content-Type: image/jpeg\nContent-length: " + length
		}
		Frame.BoundaryHeader = boundaryHeader
		Frame.Framerate = framerate
		Session.FuzzedFrames = append(Session.FuzzedFrames, Frame)
		//fmt.Printf("added file: %s", files[file])
	}
}
func handler(w http.ResponseWriter, r *http.Request) {
	/*
	var Session FuzzedSession
	if playmode {
		//load session from session.***.json file specified
		dat, err := ioutil.ReadFile(sessionpath)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(dat, &Session); err != nil {
			panic(err)
		}
		fmt.Printf("Loaded session from: %s", sessionpath)
	} else {
		fmt.Printf("Creating new session")
		createSession(&Session)
	}
*/

	//set header to multipart and describe the boundary name to be used elsewhere
	data := &bytes.Buffer{}
	data.Write([]byte(Session.ResponseHeader))
	w.Header().Write(data)
	data.Write([]byte{'\r', '\n'})
	w.Header().Set("Content-Type", "multipart/x-mixed-replace;boundary="+Session.Boundary)
	//load frames from session
	for Frame := range Session.FuzzedFrames {
		dat, err := ioutil.ReadFile(Session.FuzzedFrames[Frame].Filepath)
		if err != nil {
			panic(err)
		}
		//Write Boundary
		w.Write([]byte("\n--" + Session.Boundary + "\n"))
		w.Write([]byte(Session.FuzzedFrames[Frame].BoundaryHeader))
		w.Write([]byte("\r\n\r\n"))

		// fuzz jpeg contents

		l := len(dat)
		ratio := 0.05
		j := rand.Intn(int(float64(l)*ratio))
		//fmt.Printf("%d %d\n", l, j)

		// create random sequence
		//fmt.Printf("init seq\n")
		seq := make([]int, l)
		for i := 0; i < l; i++ {
			seq[i] = i
			//fmt.Printf("%d\n", seq[i])
		}
		rand.Shuffle(len(seq), func(i, j int) {seq[i], seq[j] = seq[j], seq[i]})

		// change selected bytes to random values
		for i := 0; i < j; i++ {
			rnd := make([]byte, 1)
			//rand.Seed(time.Now().UnixNano())
			rnd[0] = byte(rand.Intn(256))
			fmt.Printf("%d %d %d=%d\n", i, seq[i], dat[seq[i]], rnd[0])
			dat[seq[i]] = rnd[0]
		}

		//Write Image
		_, err = w.Write(dat)
		if err != nil {
			log.Println(err)
			break
		}
		fmt.Printf("sent file: %s\n", Session.FuzzedFrames[Frame].Filepath)

		//Wait
		time.Sleep(time.Minute / time.Duration(Session.FuzzedFrames[Frame].Framerate))

	}
}
func init() {
	//define command line flags
	flag.StringVar(&folderpath, "folderpath", "./1.mjpeg", "Location of jpeg files to be sent (in natural ascending order) to clients. Default: ./1.mjpeg/")
	flag.StringVar(&boundary, "boundary", "<boundary-name>", "Name of the boundary used between frames. Default: '<boundary-name>'")
	flag.IntVar(&framerate, "framerate", 600, "Framerate in frames per minute. Default: 600 i.e. 10fps")
	flag.StringVar(&hostname, "hostname", "localhost", "Hostname. Default: 'localhost'")
	flag.IntVar(&port, "port", 8080, "Serving port Default: 8080")
	flag.StringVar(&sessionpath, "sessionpath", "./", "Location of saved session json files. Default: current directory")
	flag.BoolVar(&playmode, "playmode", false, "Playback Switch. If this is set, the sessionpath is used to load a session.*.json file follows its instructions. Default: false")
	flag.BoolVar(&generatemode, "generatemode", false, "Generate Session Switch. If this is set, it generates *.json session files into the path defined in sessionpath. Default: false")
	flag.BoolVar(&fuzzmode, "fuzzmode", false, "Fuzzing Switch. If this is set, All params are ignored (except folderpath) Default: false")
	//define generate command line flags
	flag.IntVar(&generate_count, "generate_count", 1, "Generator Only: number of files generated in target sessionpath. Default: 1")
	//define fuzzer command line flags
	flag.Int64Var(&fuzz_maxlength, "fuzz_maxlength", math.MaxInt64, "Fuzzer Only: maximum reported frame length")
	flag.IntVar(&fuzz_chance_badboundry, "fuzz_chance_badboundry", 25, "Fuzzer Only: Chance of using an external boundary from response.txt files. Default: 25")
	flag.StringVar(&fuzz_datapath, "fuzz_datapath", "./1.mjpeg", "Fuzzer Only: Location of fuzzed response.txt and boundary.txt files to be sent to clients. Default: ./1.mjpeg/")
	rand.Seed(time.Now().Unix())
}
func main() {
	flag.Parse()
	createSession(&Session)
	if generatemode {
		for i := 0; i < generate_count; i++ {
			var Session FuzzedSession
			createSession(&Session)
			saveSession(&Session)
		}
	} else {
		http.HandleFunc("/", handler)
		if port > 65535 || port < 0 {
			fmt.Printf("bad port")
			return
		}
		fulladdr := net.JoinHostPort(hostname, strconv.Itoa(port))
		log.Fatal(http.ListenAndServe(fulladdr, nil))
	}
}
