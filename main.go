package main

import (
    "net/http"
    "math/rand"
    "net/url"
    "github.com/garyburd/redigo/redis"
    "flag"
    "encoding/json"
    "strings"
    "fmt"
    "time"
   // "github.com/ivpusic/golog"
)

const (
    letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456790"
    PARAM_URL = "u"     // the real url contains paramters 
    PARAM_INTERVAL = "t"  // the key will repire in X seconds 
    //PARAM_ADD ="add" // you can add more paramters after the real  url
    HASH ="h"
    PREFIX="SHORT_"
)

var (
    redisAddress   = flag.String("redis-address", "10.37.5.110:6479", "Address to the Redis server")
    maxConnections = flag.Int("max-connections", 10, "Max connections to Redis")
    httpListen = flag.String("http-listen", ":8182", "HTTP listen string")
)

type (
    Redirect struct {}
    NewMapping struct {}
    GetMapping struct{}
)

func RandStringBytes(n int, pool *redis.Pool) string {
    // http://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang

    b := make([]byte, n)
    for i := range b {
        b[i] = letterBytes[rand.Int63() % int64(len(letterBytes))]
    }

    hash := string(b)

    // check if this hash was already used
    xh := make(chan string)
    go find(hash, pool, xh)
    existingUrl := <-xh

    if existingUrl != "" {
        return RandStringBytes(n, pool)
    }

    return hash
}

func find(key string, pool *redis.Pool, ch chan string) {
    c := pool.Get()
    defer c.Close()

    value, err := redis.String(c.Do("GET", key))

    if err == nil {
        ch <- value

    } else {
        ch <- ""
    }
}

func create(url string, interval string, pool *redis.Pool, ch chan string) {
    c := pool.Get()
    defer c.Close()

    // check if this url already exists
    xh := make(chan string)
    go find(url, pool, xh)
    hash := <-xh

    if hash == "" {
        hash = RandStringBytes(6, pool)

        // @todo redis pipelining
        // @todo make sure its actually recorded

        c.Do("SET", PREFIX+hash, url)
        // record the opposite for quick check
        c.Do("SET", url, PREFIX+hash)

       if(interval!=""){
        c.Do("EXPIRE",PREFIX+hash,interval)
        c.Do("EXPIRE",url,interval)
       }
    }

    ch <- hash

    fmt.Println("generate mapping url="+url+",hash="+hash)
}

//根据hash值获取url,并且追加额外参数
func getUrlFromHash(hash string) string{

    redisPool := redis.NewPool(func() (redis.Conn, error) {
        c, err := redis.Dial("tcp", *redisAddress)

        if err != nil {
            return nil, err
        }

        return c, err
    }, *maxConnections)

    xh := make(chan string)
    go find(PREFIX+hash, redisPool, xh)
    url := <-xh

    defer redisPool.Close()

   return url
}

// func UrlEncoded(str string) (string, error) {
//     u, err := url.QueryEscape(str)
//     if err != nil {
//         return "", err
//     }
//     return u.String(), nil
// }

//#########################
// 通过短连接跳转到长连接
//传递参数列子： http://localhost:8182/redirect?h=http://tt.t/SzE7qW?aa=bb&cc=dd 

func (h *Redirect) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    t1 := time.Now().UnixNano()
     path  := r.URL.Path
     hash := path[1:len(path)]
     argss := r.URL.Query()

     fmt.Println("#######Redirect hash",hash)
     fmt.Println("#######Redirect parameters",argss)

    // 参数过滤
     if(len(hash)<=0){
         w.Header().Add("Content-Type", "application/json")
         json.NewEncoder(w).Encode(map[string]interface{}{ "error": "please input hash value with parameter like :h=xxx " })
         return 
     }


     //第一个获取到的参数h=http://tt.t/SzE7qW
     //firstargs := argss[HASH][0]
     var param_add string =""

     for key,value := range argss{
        param_add = param_add+"&"+key+"="+value[0]
     }

     if(len(param_add)>0){
        param_add = param_add[1:len(param_add)]
     }

    fmt.Println("#######Redirect param_add",param_add)
    

    url := getUrlFromHash(hash)

        //判断是否获取到hash对应的值
    fmt.Println("get original url",url)
    if(url==""){
         w.Header().Add("Content-Type", "application/json")
         json.NewEncoder(w).Encode(map[string]interface{}{ "error": "the original url is not exit or expire for hash=" +hash})
         return 
    }

   if(param_add!=""){
      if(strings.Contains(url,"?")){
        url += "&"+param_add
      }else{
        url = url +"?"+param_add
      }
   }

    http.Redirect(w, r, url, 301)

     t2 := time.Now().UnixNano()
     fmt.Println("----------Redirect cost",t2-t1)
}


//##################### 通过短链接获取长连接 
//** 传递参数列子： http://localhost:8182/get?h=http://tt.t/SzE7qW?aa=bb&cc=dd 
//
func (h *GetMapping) ServeHTTP(w http.ResponseWriter, r *http.Request) {
      t1 := time.Now().UnixNano()
   // hash := r.RequestURI[1:len(r.RequestURI)]
     argss := r.URL.Query()

     fmt.Println("#######GetMapping args",argss)

    // 参数过滤
     if(len(argss[HASH])<=0){
         w.Header().Add("Content-Type", "application/json")
         json.NewEncoder(w).Encode(map[string]interface{}{ "error": "please input hash value with parameter like :h=xxx " })
         return 
     }
     
     //第一个获取到的参数h=http://tt.t/SzE7qW
     firstargs := argss[HASH][0]
     var hash = ""
     var param_add string =""
     
     if(strings.Contains(firstargs,"?")){
       s := strings.Split(firstargs,"?")
       shorturl :=s[0]
       hash = shorturl[len(shorturl)-6:len(shorturl)]
       param_add = s[1]

    for key,value := range argss{
        //fix bug: 如果url带有多个参数，默认只识别第一个参数，这里补充多余的参数
         if(key!=HASH){
            param_add = param_add+"&"+key+"="+value[0]
          }
        }
     }else{
        hash = firstargs[len(firstargs)-6:len(firstargs)]
     }

    fmt.Println("#######GetMapping hash",hash)
    fmt.Println("#######GetMapping param_add",param_add)

    urlpath := getUrlFromHash(hash)
     
      //判断是否获取到hash对应的值
    fmt.Println("get original url",urlpath)
    if(urlpath==""){
         w.Header().Add("Content-Type", "application/json")
         json.NewEncoder(w).Encode(map[string]interface{}{ "error": "the original url is not exit or expire for hash=" +hash})
         return 
    }
   
   //拼装额外的参数
   if(param_add!=""){
      if(strings.Contains(urlpath,"?")){
        urlpath += "&"+param_add
      }else{
        urlpath = urlpath +"?"+param_add
      }
   }
    
    // fix bug: json会对特殊字符做处理，所在在json编码之前对特殊字符先做url编码
    encodeurl := url.QueryEscape(urlpath)
    fmt.Println("#######GetMapping args",encodeurl)
    w.Header().Add("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{ "url":encodeurl})
    //http.Redirect(w, r, url, 301)

     t2 := time.Now().UnixNano()
     fmt.Println("----------Redirect cost",t2-t1)
}



//#################################长连接获取短连接##################3
func (h *NewMapping) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    t1 := time.Now().UnixNano()
    args := r.URL.Query()

     fmt.Println("########NewMapping args",args)

    // 参数过滤
     if(len(args[PARAM_URL])<=0){
         w.Header().Add("Content-Type", "application/json")
         json.NewEncoder(w).Encode(map[string]interface{}{ "error": "please input original url with parameter like u=XXX" })
         return 
     }

    url := args[PARAM_URL][0]
    //fix bug: 如果url带有多个参数，这里只识别第一个参数 比如http://wiki.qdingnet.com/pages/diffpagesbyversion.action?pageId=524517&selectedPageVersions=37
    for key,value := range args{
        if(key!=PARAM_URL&&key!=PARAM_INTERVAL){
            url = url+"&"+key+"="+value[0]
        }
    }

    redisPool := redis.NewPool(func() (redis.Conn, error) {
        c, err := redis.Dial("tcp", *redisAddress)

        if err != nil {
            return nil, err
        }

        return c, err
    }, *maxConnections)

     var interval string =""
    if(len(args[PARAM_INTERVAL])>0){
        interval = args[PARAM_INTERVAL][0]
    }
    

    xh := make(chan string)
    go create(url,interval,redisPool, xh)
    hash := <-xh

    w.Header().Add("Content-Type", "application/json")

    if hash != "" {
        json.NewEncoder(w).Encode(map[string]interface{}{ "hash": strings.Replace(hash,PREFIX,"",-1)})

    } else {
        json.NewEncoder(w).Encode(map[string]interface{}{ "error": "cannot create new entry for: " + url})
    }

    defer redisPool.Close()

    t2 := time.Now().UnixNano()
     fmt.Println("----------NewMapping cost",t2-t1)
}



func main() {
    flag.Parse();

    http.Handle("/", new(Redirect))
    http.Handle("/new", new(NewMapping))
    http.Handle("/get", new(GetMapping))

    http.ListenAndServe(*httpListen, nil)
}


