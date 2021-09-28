logrus_firehose
====

 [![GoDoc](https://godoc.org/github.com/bearmini/logrus_firehose?status.svg)](https://godoc.org/github.com/bearmini/logrus_firehose)


# AWS Firehose Hook for Logrus <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/>

## Usage

```go
import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
    "github.com/bearmini/logrus_firehose"
    "github.com/sirupsen/logrus"
)

func main() {
    cred := credentials.NewCredentials(&ec2rolecreds.EC2RoleProvider{})
    awsConfig := &aws.Config{
        Credentials: cred,
        Region:      aws.String("us-west-2"),
    }
    hook, err := logrus_firehose.NewWithAWSConfig("my_stream", awsConfig)

    // set custom fire level
    hook.SetLevels([]logrus.Level{
        logrus.PanicLevel,
        logrus.ErrorLevel,
    })

    // ignore field
    hook.AddIgnore("context")

    // add custome filter
    hook.AddFilter("error", logrus_firehose.FilterError)


    // send log with logrus
    logger := logrus.New()
    logger.Hooks.Add(hook)
    logger.WithFields(f).Error("my_message") // send log data to firehose as JSON
}
```


## Special fields

Some logrus fields have a special meaning in this hook.

|||
|:--|:--|
|`message`|if `message` is not set, entry.Message is added to log data in "message" field. |
|`stream_name`|`stream_name` is the stream name for Firehose. If not set, `defaultStreamName` is used as stream name.|