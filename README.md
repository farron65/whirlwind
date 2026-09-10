# Whirlwind

Whirlwind is a basic http load testing tool

## Installation
```
git clone https://github.com/farron65/whirlwind
cd whirlwind

go build
.\whirlwind -url https://httpbin.org/get
```

## Usage

```
Usage: whirlwind -url <url> [options...]

Options:
  -url  Target URL to send requests to. Required.
  -n    Number of requests to send. Default is 10.
        Ignored if -z is set.
  
  -c    Number of concurrent workers. Default is 4.
  -qps  Queries per second (shared across all workers). Default is 10.
  -z    Duration to run the test for (e.g. 10s, 3m). If set, -n is ignored.
```

## Examples

Make requests with default settings:
```
whirlwind -url https://httpbin.org/get
```

Make 1000 requests with 100 concurrent workers:
```
whirlwind -n 1000 -c 100 -url https://httpbin.org/get
```

Run load test for 30 seconds:
```
whirlwind -z 30s -url https://httpbin.org/get
```

Rate limit to 10 queries per second:
```
whirlwind -qps 10 -c 5 -url https://httpbin.org/get
```