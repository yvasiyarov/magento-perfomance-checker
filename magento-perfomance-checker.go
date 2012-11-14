package main

import (
    "github.com/ziutek/mymysql/mysql"
    _ "github.com/ziutek/mymysql/thrsafe"
    "fmt"
    "time"
    "io/ioutil"
    "net/http"
)

type RequestInfo struct {
    Url string
    Duration Time
    IsFailed bool
    ResponseCode int
    Proto      string
    ContentLength int64
}

func NewRequestInfo(url string) *RequestInfo {
    r := new(RequestInfo)
    r.Url = url
    return r
}

func (this *RequestInfo) makeRequest() {
    start := time.Now()
    this.IsFailed = true

    resp, err := http.Get(this.url) 

    this.ResponseCode = resp.StatusCode
    this.Proto = resp.Proto
    this.ContentLength = resp.ContentLength

    if err == nil {
        if _, err := ioutil.ReadAll(resp.Body); err == nill {
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
            fmt.Print("%s %d  %v secs %d bytes ==> %s\n", this.Proto, this.StatusCode, this.Duration, this.ContentLength, this.Url)
        }
    }

    return body, err
}

func main() {
     
}
