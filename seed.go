
func seed() {
	region := "VN"

	user1 := User{
		Username:     "user1",
		Avatar:       "avatar1",
		Level:        1,
		HighestScore: 100,
	}

	user2 := User{
		Username:     "user2",
		Avatar:       "avatar2",
		Level:        2,
		HighestScore: 200,
	}

	regionAttribute := RegionAttribute{
		Population: 5,
		OnlineUsers: []User{
			user1, user2,
		},
		TotalToken: 100,
	}

	regionItem := Item{
		Region:     region,
		SK:         "REGION#" + region,
		Attributes: regionAttribute,
	}

	err := CreateItem("hello-terraform", regionItem)
	if err != nil {
		log.Fatalf("failed to create item: %v", err)
	}

	playerItems := []Item{
		{
			Region: region,
			SK:     "PLAYER#user1",
			Attributes: PlayerAttribute{
				User: user1,
				Characters: []string{
					"character1", "character2",
				},
			},
		},
		{
			Region: region,
			SK:     "PLAYER#user2",
			Attributes: PlayerAttribute{
				User: user2,
				Characters: []string{
					"character3", "character4",
				},
			},
		},
	}

	// Create a new AWS session
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := dynamodb.New(sess)

	// Prepare the batch write input
	writeRequests := []*dynamodb.WriteRequest{}
	for _, item := range playerItems {
		playerAttr, ok := item.Attributes.(PlayerAttribute)
		if !ok {
			log.Fatalf("failed to cast item.Attributes to PlayerAttribute")
		}
		characters, ok := playerAttr.Characters.([]string)
		if !ok {
			log.Fatalf("failed to cast playerAttr.Characters to []string")
		}
		charactersList := make([]*dynamodb.AttributeValue, len(characters))
		for i, character := range characters {
			charactersList[i] = &dynamodb.AttributeValue{S: aws.String(character)}
		}

		dynamoItem := map[string]*dynamodb.AttributeValue{
			"region": {
				S: aws.String(item.Region),
			},
			"sk": {
				S: aws.String(item.SK),
			},
			"attributes": {
				M: map[string]*dynamodb.AttributeValue{
					"username": {
						S: aws.String(item.Attributes.(PlayerAttribute).Username),
					},
					"avatar": {
						S: aws.String(item.Attributes.(PlayerAttribute).Avatar),
					},
					"level": {
						N: aws.String(fmt.Sprintf("%d", item.Attributes.(PlayerAttribute).Level)),
					},
					"highest_score": {
						N: aws.String(fmt.Sprintf("%d", item.Attributes.(PlayerAttribute).HighestScore)),
					},
					"characters": {
						L: charactersList,
					},
				},
			},
		}

		itemSize := calculateItemSize(dynamoItem)
		fmt.Printf("Item size: %d bytes\n", itemSize)

		writeRequests = append(writeRequests, &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: dynamoItem,
			},
		})
	}

	batchWriteInput := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			"hello-terraform": writeRequests,
		},
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	// Perform the batch write
	batchWriteOutput, err := svc.BatchWriteItem(batchWriteInput)
	if err != nil {
		log.Fatalf("failed to batch write items: %v", err)
	}

	// Print the consumed capacity
	for _, consumedCapacity := range batchWriteOutput.ConsumedCapacity {
		fmt.Printf("Table: %s, Consumed WCU: %f\n", *consumedCapacity.TableName, *consumedCapacity.CapacityUnits)
	}
}