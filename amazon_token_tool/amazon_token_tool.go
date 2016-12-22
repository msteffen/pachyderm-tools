package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"
)

type Credentials struct {
	Expiration      string
	AccessKeyId     string
	SessionToken    string
	SecretAccessKey string
}

type parent struct {
	Credentials Credentials
}

const (
	awsCredsFile = "./amazon_creds.txt"

	// for a further guide on golang time formats, see
	// http://stackoverflow.com/questions/20234104/how-to-format-current-time-using-a-yyyymmddhhmmss-format
	timeFormat = "2006-01-02T15:04:05Z0700"
)

var (
	twelveH, _ = time.ParseDuration("12h") // const
)

// Generates fake amazon creds for testing. This really should be in a separate _test file
func getFakeAmazonCredsForTesting(r io.Writer) error {
	fmt.Println("aws sts get-session-token")
	_, err := r.Write([]byte(fmt.Sprintf(`{
    "Credentials": {
        "Expiration": "%s",
        "AccessKeyId": "ACCESS-KEY-ID",
				"SessionToken": "SECRET-TOKEN",
        "SecretAccessKey": "SECRET-ACCESS-KEY"
    }
}`, time.Now().Add(twelveH).Format(timeFormat))))
	if err != nil {
		return fmt.Errorf("Could not write creds to file (%s): %s", awsCredsFile, err)
	}
	return nil
}

// Check if aws creds file exists. If not, create it and run
// 'aws sts get-session-token' to populate it
func maybeGetNewAmazonCreds() error {
	_, err := os.Stat(awsCredsFile)
	if os.IsNotExist(err) {
		newCredsFile, err := os.Create(awsCredsFile)
		defer newCredsFile.Close()
		amazonStsCmd := exec.Command("aws", "sts", "get-session-token")
		amazonStsCmd.Stdout = newCredsFile
		err = amazonStsCmd.Run()
		if err != nil {
			return fmt.Errorf("Could not write creds to file (%s): %s", awsCredsFile, err)
		}
		if err != nil {
			return fmt.Errorf("Could not create new aws creds file (%s): %s", awsCredsFile, err)
		}
	} else if err != nil {
		return fmt.Errorf("Could not stat aws creds file (%s): %s", awsCredsFile, err)
	}
	return nil
}

// Returns fresh amazon creds. May call 'aws sts get-session-token'
func getAmazonCreds() (*Credentials, error) {
	// Make sure creds file exists and is populated (contents might be old)
	maybeGetNewAmazonCreds()

	// Read and parse the contents of the creds file
	buf, err := ioutil.ReadFile(awsCredsFile)
	if err != nil {
		return nil, fmt.Errorf("Could not read aws creds file (%s): %s", awsCredsFile, err)
	}
	p := new(parent)
	err = json.Unmarshal(buf, &p)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal json in creds file: %s", err)
	}
	c := &p.Credentials

	// If the credentials in the creds file are old, delete it and start over
	expiration, err := time.Parse(timeFormat, c.Expiration)
	if err != nil {
		return nil, fmt.Errorf("Could not parse expiration time (%s) in creds file: %s", c.Expiration, err)
	}
	if time.Now().After(expiration) {
		os.Remove(awsCredsFile)
		return getAmazonCreds()
	}
	return c, nil
}

func main() {
	c, err := getAmazonCreds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
	}
	fmt.Printf("%s %s %s\n", c.AccessKeyId, c.SecretAccessKey, c.SessionToken)
}
