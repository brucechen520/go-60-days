# Topic
### dup.go
- execution
```bash
go run ./dup.go a.txt
```
- results
```bash
 ~/Documents/code/go-60-days | on main !3 ?4  go run Day03/dup.go Day03/a.txt                                                          ok | at 21:15:55
6       a
3       s
3       3
4       4
2       w
2       e
2       2
```

### gif.go
- execution
```bash
go run ./gif.go > ./lissajous.gif
```
- results
```bash
output a lissajous.gif file
```

### fetch.go
- execution
```bash
go run ./fetch.go https://www.google.com
```
- results
```bash
 ~/Documents/code/go-60-days | on main !3 ?4  go run Day03/fetch.go https://www.google.com                                             ok | at 21:17:56
<!doctype html><<html itemscope="" itemtype="http://schema.org/WebPage" lang="zh-TW"><head><meta content="text/html; charset=UTF-8" http-equiv="Content-Type"><meta content="/images/branding/googleg/1x/goo
...
```

### fetchall.go
- execution
```bash
go run Day03/fetchall.go https://www.google.com https://yahoo.com.tw
```
- results
```bash
0.19s    17221  https://www.google.com
0.25s       23  https://yahoo.com.tw
0.25s elapsed
```