# github pagerduty tf provider metrics prototyping project


## Requirements

* Env var `GHTOKEN`


## How to run the program...

```sh
go run main.go PagerDuty 'terraform-provider-pagerduty'
```

### To output the result formatted as CSV file pass the flag `csv` at the end

```sh
go run main.go PagerDuty 'terraform-provider-pagerduty' csv
```
