//go:build ignore
// +build ignore
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-west-2"),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create an S3 client
	s3Client := s3.NewFromConfig(cfg)

	// Example S3 operation: List buckets
	listBucketsOutput, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		log.Printf("failed to list S3 buckets, %v", err)
	} else {
		fmt.Println("S3 Buckets:")
		for _, bucket := range listBucketsOutput.Buckets {
			fmt.Printf("- %s, created on %s\n", *bucket.Name, *bucket.CreationDate)
		}
	}

	// Create a DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Example DynamoDB operation: List tables
	listTablesOutput, err := dynamoClient.ListTables(context.TODO(), &dynamodb.ListTablesInput{
		Limit: aws.Int32(10),
	})
	if err != nil {
		log.Printf("failed to list DynamoDB tables, %v", err)
	} else {
		fmt.Println("\nDynamoDB Tables:")
		for _, tableName := range listTablesOutput.TableNames {
			fmt.Printf("- %s\n", tableName)
		}
	}

	fmt.Println("\nAWS SDK initialized successfully!")
}

