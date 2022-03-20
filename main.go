package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/docker/docker/client"
	"os"
	"os/signal"
	"syscall"

	"time"
)

var (
	cwl           *cloudwatchlogs.CloudWatchLogs
	logGroupName  = ""
	logStreamName = ""
	sequenceToken = ""
)

func initFunc(awsAccessKeyId, awsSecretAccessKey, awsRegion string) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Credentials: credentials.NewStaticCredentialsFromCreds(credentials.Value{
				AccessKeyID:     awsAccessKeyId,
				SecretAccessKey: awsSecretAccessKey,
			}),
			Region: &awsRegion,
		},
	})
	if err != nil {
		panic(err)
	}

	cwl = cloudwatchlogs.New(sess)
	err = ensureLogGroupExists()
	if err != nil {
		panic(err)
	}
}

func main() {

	var dockerImage string
	var bashCommand string
	var cloudwatchGroup string
	var cloudwatchStream string
	var awsAccessKeyId string
	var awsSecretAccessKey string
	var awsRegion string
	flag.StringVar(&dockerImage, "docker-image", "", "--docker-image param")
	flag.StringVar(&bashCommand, "bash-command", "", "--bash-command")
	flag.StringVar(&cloudwatchGroup, "cloudwatch-group", "", "--cloudwatch-group")
	flag.StringVar(&cloudwatchStream, "cloudwatch-stream", "", "--cloudwatch-stream ")
	flag.StringVar(&awsAccessKeyId, "aws-access-key-id", "", "--aws-access-key-id")
	flag.StringVar(&awsSecretAccessKey, "aws-secret-access-key", "", "--aws-secret-access-key")
	flag.StringVar(&awsRegion, "aws-region", "", "--aws-region")
	flag.Parse()

	//set global logGroupName and logStreamName
	logGroupName = cloudwatchGroup
	logStreamName = cloudwatchStream

	initFunc(awsAccessKeyId, awsSecretAccessKey, awsRegion)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	SetupCloseHandler(cli)

	queue := make(chan string)
	go runDocker(cli, queue, dockerImage, bashCommand)
	go processQueueToCloudWatchLogs(queue)

	for {
		//var limit = int64(20)
		var startFromHead = false
		resp, err := cwl.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
			//Limit:         &limit,
			LogGroupName:  &logGroupName,
			LogStreamName: &logStreamName,
			StartFromHead: &startFromHead,
		})
		if err != nil {
			resp.String()
		}
		resp1, err := cwl.DescribeLogGroups(&cloudwatchlogs.DescribeLogGroupsInput{})
		resp1.String()

		resp2, err := cwl.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: &logGroupName,
		})
		resp2.String()
	}

}

func ensureLogGroupExists() error {
	resp, err := cwl.DescribeLogGroups(&cloudwatchlogs.DescribeLogGroupsInput{})

	if err != nil {
		return err
	}

	for _, logGroup := range resp.LogGroups {
		if *logGroup.LogGroupName == logGroupName {
			return nil
		}
	}

	_, err = cwl.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: &logGroupName,
	})
	if err != nil {
		return err
	}

	_, err = cwl.PutRetentionPolicy(&cloudwatchlogs.PutRetentionPolicyInput{
		RetentionInDays: aws.Int64(14),
		LogGroupName:    &logGroupName,
	})
	return err
}

func ensureLogStreamExists() (string, error) {
	resp, err := cwl.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: &logGroupName,
	})

	if err != nil {
		return "", err
	}

	for _, logStream := range resp.LogStreams {
		if *logStream.LogStreamName == logStreamName {
			sequenceToken = *logStream.UploadSequenceToken
			return *logStream.UploadSequenceToken, nil
		}
	}

	_, err = cwl.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &logGroupName,
		LogStreamName: &logStreamName,
	})
	return "", err
}

func processQueueToCloudWatchLogs(queue <-chan string) error {
	var logQueue []*cloudwatchlogs.InputLogEvent
	for {
		item, isOpen := <-queue
		if isOpen == false {
			return nil
		}

		logQueue = append(logQueue, &cloudwatchlogs.InputLogEvent{
			Message:   &item,
			Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
		})

		if len(logQueue) > 0 {
			input := cloudwatchlogs.PutLogEventsInput{
				LogEvents:    logQueue,
				LogGroupName: &logGroupName,
			}

			if sequenceToken == "" {
				sToken, err := ensureLogStreamExists()
				if err != nil {
					panic(err)
				}
				if sToken != "" {
					sequenceToken = sToken
					input = *input.SetSequenceToken(sequenceToken)
				}
			} else {
				input = *input.SetSequenceToken(sequenceToken)
			}

			input = *input.SetLogStreamName(logStreamName)

			resp, err := cwl.PutLogEvents(&input)
			if err != nil {
				panic(err)
			}

			if resp != nil {
				sequenceToken = *resp.NextSequenceToken
			}

			logQueue = []*cloudwatchlogs.InputLogEvent{}
		}
	}
}

func SetupCloseHandler(cli *client.Client) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		stopAndRemoveContainer(cli)
		os.Exit(0)
	}()
}
