magento-perfomance-checker
==========================
Performance measuring tools for Magento Commerce based stores. 
The main aim of this tool is to collect kind of "perfomance index" for store, in order to understand is store 
became faster or slower than before. 
This tools is emulate customers activity on typical e-commerce site - it open every 
category/product pages and log response time, status and content length in the format like siege:

HTTP/1.1 200  162.988ms 47355 bytes ==> http://www.beta.butik.ru/zhenskaja-odezhda/topy-i-majki/    
HTTP/1.1 200  173.232ms 48667 bytes ==> http://www.beta.butik.ru/zhenskaja-odezhda/brjuki/     
HTTP/1.1 200  166.899ms 47204 bytes ==> http://www.beta.butik.ru/zhenskaja-odezhda/brjuki/kapri/    
...

After completing test it displays aggregated statistics for each page type (product/category) and for all pages:
Product    
Transactions: 262 hits    
Availability: 94.66 %    
Elapsed time: 14m20.925383s     
Data transferred: 11698965 bytes    
Response time: 3.285974744s    
Transaction rate: 0.30     
Successful transactions: 248   
Failed transactions: 0     
HTTP error transactions: 14     
Longest transaction: 15.342011s      
Shortest transaction: 85.219ms     

Category     
Transactions: 256 hits    
Availability: 100.00 %    
Elapsed time: 12m7.691699s     
Data transferred: 11154863 bytes    
Response time: 2.842545699s    
Transaction rate: 0.35    
Successful transactions: 256    
Failed transactions: 0    
HTTP error transactions: 0    
Longest transaction: 15.229138s     
Shortest transaction: 72.136ms     
   
Total stats    
Transactions: 518 hits    
Availability: 97.30 %    
Elapsed time: 26m28.617082s      
Data transferred: 22853828 bytes     
Response time: 3.066828343s    
Transaction rate: 0.33    
Successful transactions: 504    
Failed transactions: 0    
HTTP error transactions: 14     
Longest transaction: 15.342011s     
Shortest transaction: 72.136ms      
   
# Installation   
    Install Go (http://golang.org/doc/install#install)   
    go build magento-perfomance-checker.go   
    
# Usage    
./magento-perfomance-checker -h    
Usage of ./magento-perfomance-checker:    
  -magento_base_url="": Magento base url    
  -magento_database="magento": Magento database    
  -mysql_host="localhost": MySQL host     
  -mysql_login="root": MySQL login     
  -mysql_password="root": MySQL password     
  -mysql_port=3306: MySQL port    
  -no_connections=50: Number of parallel connections    
  -num_cpu=1: Number of used CPU    
