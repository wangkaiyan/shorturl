# golang 开发的短链接服务
短链接服务是指把普通的长连接（带有参数）转化为6个字符和数字组成的网址，以http协议提供服务
包含2接口:
1 输入长链接生成短链接服务接口
http://devst.xxxx.com/new?u=http://www.xxx.com?xx=yyy&t=100
  u （必填字段， 注意在传输之前需要用urlencode，防止特殊字符错误）： url 标识完整的长连接url 可以包含多个参数  
  t （可选字段）:  time 标识 生成的短链接有效时长，单位是秒， 比如100标识生成的短链接有效期是100miao，
                              如果不传该参数 默认标识 永久有效。
 正常resposne：  以json格式返回 {"hash":"YWx7Y0"} 标识生成的的短链接字符是YWx7Y0 （固定6个字符）
失败的response：  以json格式返回 {"error":"please input original url with parameter like u=XXX"}
 
2 短链接跳转服务：http://devst.xxxx.com/PauYe9?key=value
 
正常的response 会直接跳转到对应的长链接地址网页
错误的response会以json格式返回 
{"error":"please input hash value with parameter like :h=xxx "}
 
3 短连接获取服务
  http://devst.xxxx.com/get?h=http://devst.xxxx.com/PauYe9?key=value
  h （必填字段）： hash  标识短链接hash值
  add(可选字段)： 在原来的长连接之后继续添加额外的参数值
 
正常的response 会返回对应的url, 注意防止特殊字符错误，这里url也是经过urlencode的，需要用urldecode
 
性能： 因为短链接服务采用golang开发，目前测试100个并发线程，最大qps 800多。