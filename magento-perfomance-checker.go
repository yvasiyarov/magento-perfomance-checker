package main

import (
    "github.com/ziutek/mymysql/mysql"
    _ "github.com/ziutek/mymysql/thrsafe"
    "fmt"
    "time"
    "io/ioutil"
    "net/http"
)

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
    r.Url = url
    return r
}

func (this *RequestInfo) makeRequest() {
    start := time.Now()
    this.IsFailed = true

    resp, err := http.Get(this.Url) 

    this.ResponseCode = resp.StatusCode
    this.Proto = resp.Proto
    this.ContentLength = resp.ContentLength

    if err == nil {
        if _, err := ioutil.ReadAll(resp.Body); err == nil {
            this.IsFailed = true
        }
    }


    defer func() {
        resp.Body.Close()
        end := time.Now()
        this.Duration = end.Sub(start)
        if this.IsFailed {
            fmt.Print("Url request failed: %v\n", this.Url)
        } else {
            fmt.Print("%s %d  %v secs %d bytes ==> %s\n", this.Proto, this.ResponseCode, this.Duration, this.ContentLength, this.Url)
        }
    }()
}

func readUrls(inRequestsChanel chan *RequestInfo) error {
    db := mysql.New("tcp", "", "127.0.0.1:3306", "root", "", "magento_butik")
    fmt.Println("Connect to magento DB")
    if err := db.Connect(); err != nil {
        fmt.Println("Can not connect to magento DB")
        return err
    }
    defer func() { inRequestsChanel <- nil}()
    
    if rows, queryResult, err := db.Query("SELECT request_path, category_id, product_id FROM core_url_write WHERE options <> 'RP' LIMIT 1000"); err != nil {
        fmt.Println("Can not query urls")

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
    defer func() { outRequestsChanel <- nil}()

    for request := <- inRequestsChanel; request != nil; request = <- inRequestsChanel {
        routines <- 1
        go func(routines chan int, request *RequestInfo, outRequestsChanel chan *RequestInfo) {
            request.makeRequest()
            <-routines
            outRequestsChanel <- request
        }(routines, request, outRequestsChanel)
    }
}

func calculateStat(outRequestsChanel chan *RequestInfo) {
    totalTime := new(time.Time);
    var totalFailed int;
    var totalSuccess int;
    var totalHttpErrors int;
    var totalContentLength int64;

    if request := <- outRequestsChanel; request != nil {
        if request.IsFailed {
            totalFailed = totalFailed + 1
        } else if request.ResponseCode == 200 {
            totalSuccess = totalSuccess + 1
        } else {
            totalHttpErrors = totalHttpErrors + 1
        }
        totalContentLength = totalContentLength + request.ContentLength
        totalTime.Add(request.Duration)
    } else {
        fmt.Printf("Transactions: %d hits\n", totalFailed + totalSuccess + totalHttpErrors) 
        fmt.Printf("Availability: %f %\n", 100 - (totalFailed / (totalFailed + totalSuccess + totalHttpErrors)) * 100)
        fmt.Printf("Elapsed time: %v secs\n", totalTime) 
        fmt.Printf("Data transferred: %d bytes\n", totalContentLength)
//        fmt.Printf("Data transferred: %d bytes\n", totalContentLength)
 
    }
}

func main() {
     inRequestsChanel := make(chan *RequestInfo)
     outRequestsChanel := make(chan *RequestInfo)

     go readUrls(inRequestsChanel)
     go makeRequests(inRequestsChanel, outRequestsChanel, 50)
     calculateStat(outRequestsChanel)
}
