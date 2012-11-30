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

    fmt.Println("Before Query:" + this.Url)
    defer fmt.Println("Polundra!!")
    resp, err := http.Get(this.Url) 
//    defer resp.Body.Close()
    fmt.Println("Query:" + this.Url)

    if err == nil {
        fmt.Println("Make HTTP request")
        if _, err := ioutil.ReadAll(resp.Body); err == nil {
            this.IsFailed = false

	    this.ResponseCode = resp.StatusCode
	    this.Proto = resp.Proto
	    this.ContentLength = resp.ContentLength

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
        fmt.Println("HTTP request err")
        fmt.Println(err)
    }
}

func readUrls(inRequestsChanel chan *RequestInfo) error {
    defer close(inRequestsChanel)

    db := mysql.New("tcp", "", *mysqlHost + ":" + strconv.Itoa(*mysqlPort), *mysqlLogin, *mysqlPassword, *magentoDatabase)
    fmt.Println("Connect to magento DB")
    if err := db.Connect(); err != nil {
        fmt.Printf("Can not connect to magento DB:%v \n", err)
        return err
    }
    
    if rows, queryResult, err := db.Query("SELECT request_path, category_id, product_id FROM core_url_rewrite WHERE is_system=1 LIMIT 10"); err != nil {
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
        fmt.Println("All urls sended")
    }

    return nil
}

func makeRequests(inRequestsChanel chan *RequestInfo, outRequestsChanel chan *RequestInfo, noParallelRoutines int) {
    routines := make(chan int, noParallelRoutines)
    ticktes  := make(chan int, noParallelRoutines)
    defer close(outRequestsChanel)

    for request := range inRequestsChanel {
        if request == nil {
            break;
        }
        routines <- 1
        go func(routines chan int, request *RequestInfo, outRequestsChanel chan *RequestInfo) {
            request.makeRequest()
            outRequestsChanel <- request
            <- routines

            fmt.Println("One go routine exit")
        }(routines, request, outRequestsChanel)
    }
    close(routines)

    for _ = range routines { }
    fmt.Println("make requests exit!")
}

func calculateStat(outRequestsChanel chan *RequestInfo) {
    totalTime := new(time.Time);
    var totalFailed int;
    var totalSuccess int;
    var totalHttpErrors int;
    var totalContentLength int64;

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
        totalTime.Add(request.Duration)
    }
 
    total := totalFailed + totalSuccess + totalHttpErrors
    var availability float64
    if total != 0 {
        availability = 100.0 - float64(totalFailed / total) * 100.0
        fmt.Printf("Transactions: %d hits\n", total) 
        fmt.Printf("Availability: %s %\n", strconv.FormatFloat(availability, 'f', 2, 64))
        fmt.Printf("Elapsed time: %v secs\n", totalTime) 
        fmt.Printf("Data transferred: %d bytes\n", totalContentLength)
//        fmt.Printf("Data transferred: %d bytes\n", totalContentLength)
        
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
