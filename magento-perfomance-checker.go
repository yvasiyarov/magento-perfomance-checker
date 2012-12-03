package main

import (
    "github.com/ziutek/mymysql/mysql"
    _ "github.com/ziutek/mymysql/thrsafe"
    "fmt"
    "time"
    "io/ioutil"
    "net/http"
    "flag"
    "runtime"
    "strconv"
    "math"
)

var mysqlPort = flag.Int("mysql_port", 3306, "MySQL port")
var mysqlHost = flag.String("mysql_host", "localhost", "MySQL host")
var mysqlLogin = flag.String("mysql_login", "root", "MySQL login")
var mysqlPassword = flag.String("mysql_password", "root", "MySQL password")

var magentoDatabase = flag.String("magento_database", "magento", "Magento database")
var magentoBaseUrl  = flag.String("magento_base_url", "", "Magento base url")

var numConnections = flag.Int("no_connections", 50, "Number of parallel connections")
var numCpu = flag.Int("num_cpu", 1, "Number of used CPU")

type UrlType int

const (
    UrlTypeProduct UrlType = iota + 1
    UrlTypeCategory
    UrlTypeUnknown
)

type RequestInfo struct {
    Url string
    Duration time.Duration
    IsFailed bool
    ResponseCode int
    Proto      string
    ContentLength int64
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

func readUrls(inRequestsChanel chan *RequestInfo) error {
    defer close(inRequestsChanel)

    db := mysql.New("tcp", "", *mysqlHost + ":" + strconv.Itoa(*mysqlPort), *mysqlLogin, *mysqlPassword, *magentoDatabase)
    if err := db.Connect(); err != nil {
        fmt.Printf("Can not connect to magento DB:%v \n", err)
        return err
    }
    
    if rows, queryResult, err := db.Query("SELECT request_path, category_id, product_id FROM core_url_rewrite WHERE is_system=1"); err != nil {
        fmt.Printf("Can not query urls: %v \n", err)

        return err
    } else {
        requestPath := queryResult.Map("request_path")
        categoryId  := queryResult.Map("category_id")
        productId   := queryResult.Map("product_id")
        
        for _, row := range rows{
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

    return nil
}

func makeRequests(inRequestsChanel chan *RequestInfo, outRequestsChanel chan *RequestInfo, noParallelRoutines int) {
    routines := make(chan int, noParallelRoutines)
    numRoutines := 0

    defer close(outRequestsChanel)

    for request := range inRequestsChanel {
        if numRoutines >= noParallelRoutines {
            <- routines
            numRoutines --
        }

        go func(routines chan int, request *RequestInfo, outRequestsChanel chan *RequestInfo) {
            request.makeRequest()
            outRequestsChanel <- request
            routines <- 1

        }(routines, request, outRequestsChanel)
        
        numRoutines++
    }

    for i := 0; i < numRoutines; i++ {
        <- routines
    }

}

func calculateStat(outRequestsChanel chan *RequestInfo) {
    var totalTime int64
    var longestTransactionTime float64
    var shortesTransactionTime float64
    var totalFailed int
    var totalSuccess int
    var totalHttpErrors int
    var totalContentLength int64

    shortesTransactionTime = math.MaxFloat64
    for request := range outRequestsChanel {
        if request == nil {
            break;
        }
        if request.IsFailed {
            totalFailed = totalFailed + 1
        } else if request.ResponseCode == 200 {
            totalSuccess = totalSuccess + 1
        } else {
            totalHttpErrors = totalHttpErrors + 1
        }
        totalContentLength = totalContentLength + request.ContentLength
        totalTime += request.Duration.Nanoseconds()

        longestTransactionTime = math.Max(longestTransactionTime, float64(request.Duration.Nanoseconds()))
        shortesTransactionTime = math.Min(shortesTransactionTime, float64(request.Duration.Nanoseconds()))
    }
 
    total := totalFailed + totalSuccess + totalHttpErrors
    var availability float64
    if total != 0 {
        availability = 100.0 - float64(totalFailed / total) * 100.0
        fmt.Printf("Transactions: %d hits\n", total) 
        fmt.Printf("Availability: %s %%\n", strconv.FormatFloat(availability, 'f', 2, 64))
        fmt.Printf("Elapsed time: %s \n", time.Duration(totalTime).String())
        fmt.Printf("Data transferred: %d bytes\n", totalContentLength)
        fmt.Printf("Response time: %s\n", time.Duration((totalTime / int64(total))).String())
        fmt.Printf("Transaction rate: %s\n", strconv.FormatFloat(float64(total) / time.Duration(totalTime).Seconds(), 'f', 2, 64))
        fmt.Printf("Successful transactions: %d\n", totalSuccess)
        fmt.Printf("Failed transactions: %d\n", totalFailed)
        fmt.Printf("HTTP error transactions: %d\n", totalHttpErrors)
        fmt.Printf("Longest transaction: %s \n", time.Duration(int64(longestTransactionTime)).String())
        fmt.Printf("Shortest transaction: %s \n", time.Duration(int64(shortesTransactionTime)).String())
        
    } else {
        fmt.Println("No requests was done")
    }

 
}

func main() {
     flag.Parse()
     runtime.GOMAXPROCS(*numCpu)

     inRequestsChanel := make(chan *RequestInfo)
     outRequestsChanel := make(chan *RequestInfo)

     go readUrls(inRequestsChanel)
     go makeRequests(inRequestsChanel, outRequestsChanel, *numConnections)
     calculateStat(outRequestsChanel)
}
