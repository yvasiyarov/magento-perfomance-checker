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

func readUrls() error {
    db := mysql.New("tcp", "", "127.0.0.1:3306", "root", "", "magento_butik")
    fmt.Println("Connect to magento DB")
    if err := db.Connect(); err != nil {
        fmt.Println("Can not connect to magento DB")
        return err
    }
    
    if rows, queryResult, err := db.Query("SELECT request_path, category_id, product_id FROM core_url_write WHERE options <> 'RP' LIMIT 1000"); err != null {
        fmt.Println("Can not query urls")
        return err
    } else {
        requestPath := queryResult.Map("request_path")
        categoryId  := queryResult.Map("category_id")
        productId   := queryResult.Map("product_id")
        
        for _, row := range rows{
            request := NewRequestInfo(row.Str(requestPath))
            if row.Int(productId) != 0 {
                requestPath.RequestUrlType = UrlTypeProduct
            } else if row.Int(categoryId) != 0 {
                requestPath.RequestUrlType = UrlTypeCategory
            } else {
                requestPath.RequestUrlType = UrlTypeUnknown
            }
        }
        
    }
    return nil
}
func main() {
     readUrls();
}
