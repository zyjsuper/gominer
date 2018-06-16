package main
/*
#cgo CFLAGS: -I .
#cgo LDFLAGS: -L . -lpthread


int callOnMeGo_cgo(int in); // Forward declaration.
*/

//import "math/rand"
//import "time"

import "C"
import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/zyjsuper/gominer/algorithms/sia"
	"github.com/zyjsuper/gominer/mining"
	"github.com/zyjsuper/go-opencl/cl"
)

//Version is the released version string of gominer
var Version = "1.0"

var intensity = 28
var devicesTypesForMining = cl.DeviceTypeGPU

func main() {
	//rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	//worker := strconv.FormatInt(int64(rnd.Int31n(1000000)),6)
	log.SetOutput(os.Stdout)
	printVersion := flag.Bool("v", false, "Show version and exit")
	useCPU := flag.Bool("cpu", false, "If set, also use the CPU for mining, only GPU's are used by default")
	flag.IntVar(&intensity, "I", intensity, "Intensity")
	host := flag.String("url", "stratum+tcp://bcx.vvpool.com:5661", "daemon or server host and port, for stratum servers, use `stratum+tcp://<host>:<port>`")
	pooluser := flag.String("user", "XNqedko5Cw6vmd2V4HTPALzW7aHK3q21xp."+"worker5", "username, most stratum servers take this in the form [payoutaddress].[rigname]")
	excludedGPUs := flag.String("E", "", "Exclude GPU's: comma separated list of devicenumbers")
	flag.Parse()

	if *printVersion {
		fmt.Println("gominer version", Version)
		os.Exit(0)
	}

	if *useCPU {
		devicesTypesForMining = cl.DeviceTypeAll
	}
	globalItemSize := int(math.Exp2(float64(intensity)))

	platforms, err := cl.GetPlatforms()
	if err != nil {
		log.Panic(err)
	}

	clDevices := make([]*cl.Device, 0, 5)
	for _, platform := range platforms {
		log.Println("Platform", platform.Name())
		platormDevices, err := cl.GetDevices(platform, devicesTypesForMining)
		if err != nil {
			log.Println(err)
		}
		log.Println(len(platormDevices), "device(s) found:")
		for i, device := range platormDevices {
			log.Println(i, "-", device.Type(), "-", device.Name())
			clDevices = append(clDevices, device)
		}
	}

	if len(clDevices) == 0 {
		log.Println("No suitable opencl devices found")
		os.Exit(1)
	}

	//Filter the excluded devices
	miningDevices := make(map[int]*cl.Device)
	for i, device := range clDevices {
		if deviceExcludedForMining(i, *excludedGPUs) {
			continue
		}
		miningDevices[i] = device
	}

	nrOfMiningDevices := len(miningDevices)
	var hashRateReportsChannel = make(chan *mining.HashRateReport, nrOfMiningDevices*10)

	var miner mining.Miner
	log.Println("----------开始挖BCX----------")
	c := sia.NewClient(*host, *pooluser)

	miner = &sia.Miner{
		ClDevices:       miningDevices,
		HashRateReports: hashRateReportsChannel,
		Intensity:       intensity,
		GlobalItemSize:  globalItemSize,
		Client:          c,
	}
	miner.Mine()

	//Start printing out the hashrates of the different gpu's
	hashRateReports := make([]float64, nrOfMiningDevices)
	for {
		//No need to print at every hashreport, we have time
		for i := 0; i < nrOfMiningDevices; i++ {
			report := <-hashRateReportsChannel
			hashRateReports[report.MinerID] = report.HashRate
		}
		fmt.Print("\r")
		var totalHashRate float64
		for minerID, hashrate := range hashRateReports {
			fmt.Printf("%d-%.1f ", minerID, hashrate)
			totalHashRate += hashrate
		}
		fmt.Printf("算力: %.1f MH/s  ", totalHashRate)

	}
}

//deviceExcludedForMining checks if the device is in the exclusion list
func deviceExcludedForMining(deviceID int, excludedGPUs string) bool {
	excludedGPUList := strings.Split(excludedGPUs, ",")
	for _, excludedGPU := range excludedGPUList {
		if strconv.Itoa(deviceID) == excludedGPU {
			return true
		}
	}
	return false
}
