package main

import (
	"flag"
	"fmt"
	"github.com/ziutek/mymysql/mysql"
	_ "github.com/ziutek/mymysql/thrsafe"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"time"
)

var mysqlPort = flag.Int("mysql_port", 3306, "MySQL port")
var mysqlHost = flag.String("mysql_host", "localhost", "MySQL host")
var mysqlLogin = flag.String("mysql_login", "root", "MySQL login")
var mysqlPassword = flag.String("mysql_password", "root", "MySQL password")

var magentoDatabase = flag.String("magento_database", "magento", "Magento database")
var magentoBaseUrl = flag.String("magento_base_url", "", "Magento base url")

var numConnections = flag.Int("no_connections", 50, "Number of parallel connections")
var numCpu = flag.Int("num_cpu", 1, "Number of used CPU")

type UrlType int

const (
	UrlTypeProduct UrlType = iota + 1
	UrlTypeCategory
	UrlTypeUnknown

	NoUrlTypes //Number of Url types. Used to iterate other constant list
)

type RequestInfo struct {
	Url            string
	Duration       time.Duration
	IsFailed       bool
	ResponseCode   int
	Proto          string
	ContentLength  int64
	RequestUrlType UrlType
}

func NewRequestInfo(url string) *RequestInfo {
	r := new(RequestInfo)
	r.Url = *magentoBaseUrl + url
	return r
}

func (this *RequestInfo) makeRequest() {
	start := time.Now()
	this.IsFailed = true

	resp, err := http.Get(this.Url)
	defer resp.Body.Close()

	if err == nil {
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			this.IsFailed = false

			this.ResponseCode = resp.StatusCode
			this.Proto = resp.Proto
			this.ContentLength = int64(len(body))

			end := time.Now()
			this.Duration = end.Sub(start)

			if this.IsFailed {
				fmt.Printf("Url request failed: %v\n", this.Url)
			} else {
				fmt.Printf("%s %d  %v %d bytes ==> %s\n", this.Proto, this.ResponseCode, this.Duration, this.ContentLength, this.Url)
			}
		} else {
			fmt.Println(err)
		}

	} else {
		fmt.Println(err)
	}
}

func readUrls(inRequestsChanel chan *RequestInfo, osSignal chan os.Signal) error {
	defer close(inRequestsChanel)

	db := mysql.New("tcp", "", *mysqlHost+":"+strconv.Itoa(*mysqlPort), *mysqlLogin, *mysqlPassword, *magentoDatabase)
	if err := db.Connect(); err != nil {
		fmt.Printf("Can not connect to magento DB:%v \n", err)
		return err
	}

	if rows, queryResult, err := db.Query("SELECT request_path, category_id, product_id FROM core_url_rewrite WHERE is_system=1"); err != nil {
		fmt.Printf("Can not query urls: %v \n", err)

		return err
	} else {
		requestPath := queryResult.Map("request_path")
		categoryId := queryResult.Map("category_id")
		productId := queryResult.Map("product_id")

		for _, row := range rows {
			select {
			case sign := <-osSignal:
				fmt.Printf("\nCatch signal %#v\n", sign)
				return nil
			default:
				request := NewRequestInfo(row.Str(requestPath))
				if row.Int(productId) != 0 {
					request.RequestUrlType = UrlTypeProduct
				} else if row.Int(categoryId) != 0 {
					request.RequestUrlType = UrlTypeCategory
				} else {
					request.RequestUrlType = UrlTypeUnknown
				}
				inRequestsChanel <- request
			}
		}
	}

	return nil
}

func makeRequests(inRequestsChanel chan *RequestInfo, outRequestsChanel chan *RequestInfo, noParallelRoutines int) {
	routines := make(chan int, noParallelRoutines)
	numRoutines := 0

	defer close(outRequestsChanel)

	for request := range inRequestsChanel {
		if numRoutines >= noParallelRoutines {
			<-routines
			numRoutines--
		}

		go func(routines chan int, request *RequestInfo, outRequestsChanel chan *RequestInfo) {
			request.makeRequest()
			outRequestsChanel <- request
			routines <- 1

		}(routines, request, outRequestsChanel)

		numRoutines++
	}

	for i := 0; i < numRoutines; i++ {
		<-routines
	}
}

type Stats struct {
	TotalTime              int64
	LongestTransactionTime float64
	ShortesTransactionTime float64
	TotalFailed            int
	TotalSuccess           int
	TotalHttpErrors        int
	TotalContentLength     int64
}

func printStat(stats *Stats, statsType UrlType) {
	total := stats.TotalFailed + stats.TotalSuccess + stats.TotalHttpErrors
	var availability float64
	if total == 0 {
		return
	}

	title := "Total stats"
	switch statsType {
	case UrlTypeProduct:
		title = "Product"
	case UrlTypeCategory:
		title = "Category"
	case UrlTypeUnknown:
		title = "Other"
	}
	fmt.Println("===================================================================")
	fmt.Printf("|| %s\n", title)
	fmt.Println("===================================================================")

	availability = 100.0 - float64(stats.TotalFailed+stats.TotalHttpErrors) / float64(total)*100.0
	fmt.Printf("Transactions: %d hits\n", total)
	fmt.Printf("Availability: %s %%\n", strconv.FormatFloat(availability, 'f', 2, 64))
	fmt.Printf("Elapsed time: %s \n", time.Duration(stats.TotalTime).String())
	fmt.Printf("Data transferred: %d bytes\n", stats.TotalContentLength)
	fmt.Printf("Response time: %s\n", time.Duration((stats.TotalTime / int64(total))).String())
	fmt.Printf("Transaction rate: %s\n", strconv.FormatFloat(float64(total)/time.Duration(stats.TotalTime).Seconds(), 'f', 2, 64))
	fmt.Printf("Successful transactions: %d\n", stats.TotalSuccess)
	fmt.Printf("Failed transactions: %d\n", stats.TotalFailed)
	fmt.Printf("HTTP error transactions: %d\n", stats.TotalHttpErrors)
	fmt.Printf("Longest transaction: %s \n", time.Duration(int64(stats.LongestTransactionTime)).String())
	fmt.Printf("Shortest transaction: %s \n", time.Duration(int64(stats.ShortesTransactionTime)).String())
}

func calculateStat(outRequestsChanel chan *RequestInfo) {

	stats := make(map[UrlType]*Stats, NoUrlTypes)
	numTypes := int(NoUrlTypes)
	for i := 1; i < numTypes; i++ {
		stats[UrlType(i)] = new(Stats)
		stats[UrlType(i)].ShortesTransactionTime = math.MaxFloat64
	}
	for request := range outRequestsChanel {
		if request == nil {
			break
		}
		if request.IsFailed {
			stats[request.RequestUrlType].TotalFailed++
		} else if request.ResponseCode == 200 {
			stats[request.RequestUrlType].TotalSuccess++
		} else {
			stats[request.RequestUrlType].TotalHttpErrors++
		}
		stats[request.RequestUrlType].TotalContentLength += request.ContentLength
		stats[request.RequestUrlType].TotalTime += request.Duration.Nanoseconds()

		stats[request.RequestUrlType].LongestTransactionTime = math.Max(stats[request.RequestUrlType].LongestTransactionTime, float64(request.Duration.Nanoseconds()))
		stats[request.RequestUrlType].ShortesTransactionTime = math.Min(stats[request.RequestUrlType].ShortesTransactionTime, float64(request.Duration.Nanoseconds()))
	}

	totalStat := new(Stats)
	for i := 1; i < numTypes; i++ {
		currentType := UrlType(i)

		totalStat.TotalFailed += stats[currentType].TotalFailed
		totalStat.TotalSuccess += stats[currentType].TotalSuccess
		totalStat.TotalHttpErrors += stats[currentType].TotalHttpErrors
		totalStat.TotalContentLength += stats[currentType].TotalContentLength
		totalStat.TotalTime += stats[currentType].TotalTime

		totalStat.LongestTransactionTime = math.Max(totalStat.LongestTransactionTime, stats[currentType].LongestTransactionTime)
		if stats[currentType].ShortesTransactionTime > 0 {
			totalStat.ShortesTransactionTime = math.Min(totalStat.ShortesTransactionTime, stats[currentType].ShortesTransactionTime)
		}

		printStat(stats[currentType], currentType)
	}

	printStat(totalStat, UrlType(0))
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*numCpu)

	inRequestsChanel := make(chan *RequestInfo)
	outRequestsChanel := make(chan *RequestInfo)
	osSignal := make(chan os.Signal, 1)

	signal.Notify(osSignal)

	go readUrls(inRequestsChanel, osSignal)
	go makeRequests(inRequestsChanel, outRequestsChanel, *numConnections)
	calculateStat(outRequestsChanel)
}
