package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type User struct {
	Username     string `json:"username"`
	Avatar       string `json:"avatar"`
	Level        int    `json:"level"`
	HighestScore int    `json:"highest_score"`
}

type RegionAttribute struct {
	Population int `json:"population"`
	// OnlineUsers []User `json:"online_users"`
	TotalToken int `json:"total_token"`
}

type PlayerAttribute struct {
	User
	Characters interface{} `json:"characters"`
}

type Item struct {
	Region     string      `json:"region"`
	SK         string      `json:"sk"`
	Attributes interface{} `json:"attributes"`
}

var svc *dynamodb.DynamoDB

func init() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc = dynamodb.New(sess)
}

func CreateItem(tableName string, item Item) error {
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %v", err)
	}

	input := &dynamodb.PutItemInput{
		Item:                av,
		TableName:           aws.String(tableName),
		ConditionExpression: aws.String("attribute_not_exists(#r) AND attribute_not_exists(sk)"),
		ExpressionAttributeNames: map[string]*string{
			"#r": aws.String("region"),
		},
	}

	result, err := svc.PutItem(input)
	if err != nil {
		return fmt.Errorf("failed to put item: %v", err)
	}

	log.Printf("PutItem result: %+v\n", result)
	return nil
}

func GetItem(tableName, pk string) (*Item, error) {
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"region": {
				S: aws.String(pk),
			},
			"sk": {
				S: aws.String("REGION#" + pk),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %v", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("item not found")
	}

	item := new(Item)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal item: %v", err)
	}

	return item, nil
}

func UpdateItem(tableName string, item Item) error {
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %v", err)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		return fmt.Errorf("failed to update item: %v", err)
	}

	return nil
}

func DeleteItem(tableName, id string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
		},
	}

	_, err := svc.DeleteItem(input)
	if err != nil {
		return fmt.Errorf("failed to delete item: %v", err)
	}

	return nil
}

func sample() {
	tableName := "hello-terraform"

	// Create a new AWS session
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			MaxRetries: aws.Int(0), // Disable retries
		},
	}))
	svc := dynamodb.New(sess)

	// WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup

	// Mutex to protect the throttled counter
	var mu sync.Mutex

	// Counter for throttled requests
	throttledCount := 0

	// Number of concurrent requests
	numRequests := 100

	// Start multiple goroutines
	for i := 0; i < numRequests; i++ {
		wg.Add(1) // Increment the WaitGroup counter

		go func(requestID int) {
			defer wg.Done() // Decrement the counter when the goroutine finishes

			getItemInput := &dynamodb.GetItemInput{
				TableName: aws.String(tableName),
				Key: map[string]*dynamodb.AttributeValue{
					"region": {
						S: aws.String("EN"),
					},
					"sk": {
						S: aws.String("REGION#EN"),
					},
				},
				ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
			}

			// Make the GetItem request
			_, err := svc.GetItem(getItemInput)
			if err != nil {
				if isThrottlingError(err) {
					mu.Lock()
					throttledCount++
					mu.Unlock()
				}
				return
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	fmt.Printf("All requests completed. Throttled requests: %d\n", throttledCount)
}

// isThrottlingError checks if the error is a throttling error
func isThrottlingError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == dynamodb.ErrCodeProvisionedThroughputExceededException {
			return true
		}
	}
	return false
}

func calculateItemSize(item map[string]*dynamodb.AttributeValue) int {
	size := 0
	for _, v := range item {
		size += calculateAttributeValueSize(v)
	}
	return size
}

// calculateAttributeValueSize calculates the size of a DynamoDB attribute value in bytes
func calculateAttributeValueSize(value *dynamodb.AttributeValue) int {
	size := 0
	if value.S != nil {
		size += len(*value.S)
	}
	if value.N != nil {
		size += len(*value.N)
	}
	if value.B != nil {
		size += len(value.B)
	}
	if value.SS != nil {
		for _, s := range value.SS {
			size += len(*s)
		}
	}
	if value.NS != nil {
		for _, n := range value.NS {
			size += len(*n)
		}
	}
	if value.BS != nil {
		for _, b := range value.BS {
			size += len(b)
		}
	}
	if value.M != nil {
		for k, v := range value.M {
			size += len(k)
			size += calculateAttributeValueSize(v)
		}
	}
	if value.L != nil {
		for _, v := range value.L {
			size += calculateAttributeValueSize(v)
		}
	}
	if value.NULL != nil {
		size += 1
	}
	if value.BOOL != nil {
		size += 1
	}
	return size
}

func main() {
	output, err := getListOnlineUser("VN")
	if err != nil {
		log.Fatalf("failed to get list online user: %v", err)
	}

	for _, user := range output {
		fmt.Printf("Username: %s, Avatar: %s, Level: %d, HighestScore: %d\n", user.Username, user.Avatar, user.Level, user.HighestScore)
	}
}

func getListOnlineUser(region string) ([]User, error) {
	item, err := GetItem("hello-terraform", region)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %v", err)
	}

	// Ensure Attributes is a map[string]interface{}
	attributesMap, ok := item.Attributes.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to cast item.Attributes to map[string]interface{}")
	}

	// Unmarshal the Attributes field into a RegionAttribute struct
	var regionAttribute RegionAttribute
	if err := mapToStruct(attributesMap, &regionAttribute); err != nil {
		return nil, fmt.Errorf("failed to unmarshal item.Attributes: %v", err)
	}

	// Extract online users
	onlineUsers, ok := attributesMap["online_users"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to cast online_users to []interface{}")
	}

	// Convert online users to []User
	users, err := convertToUsers(onlineUsers)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func mapToStruct(m map[string]interface{}, out interface{}) error {
	attributesAVMap := make(map[string]*dynamodb.AttributeValue)
	for k, v := range m {
		av, err := dynamodbattribute.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal attribute %s: %v", k, err)
		}
		attributesAVMap[k] = av
	}
	return dynamodbattribute.UnmarshalMap(attributesAVMap, out)
}

func convertToUsers(onlineUsers []interface{}) ([]User, error) {
	var users []User
	for _, u := range onlineUsers {
		userMap, ok := u.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to cast user to map[string]interface{}")
		}
		var user User
		if err := mapToStruct(userMap, &user); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user: %v", err)
		}
		users = append(users, user)
	}
	return users, nil
}
