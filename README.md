# Golang Tool

This repository is a golang tool which executes shell-script inside of docker container and sends logs to AWS CloudWatch

## Prerequisites

Before you start, please make sure you have Go installed in your system. If not, please use the following link to install Golang:
https://golang.org/doc/install

## Getting Started

Clone the git repository in your system and then cd into project root directory

```bash
$ git clone https://github.com/Hitsugaya7/golang-test-task
```

Build your tool by executing the following steps
```bash
$ go build 
```

## Sample Outputs

This tool executes shell-script inside of docker container and sends logs to AWS CloudWatch:
```bash
$ ./golang-test-task_ --docker-image python --bash-command 'echo something' --cloudwatch-group test-log-group-name-4 --cloudwatch-stream test-log-stream-name-4 --aws-access-key-id ADS123 --aws-secret-access-key QWE+123 --aws-region us-west-2


```
To stop the app enter ctr+c

```bash
- Ctrl+C pressed in Terminal
  Wait, the app is stopping and removing the docker container

```
