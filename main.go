package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
)

func main() {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/rds/#RDS.RestoreDBClusterToPointInTime
	// TODO: get latest time in format - "2006-01-02T15:04:05.999999999Z", "2016-09-13T18:45:00Z"
	// TODO: add variation in destination cluter names, maybe a list of 2-3 things, if one exists use the next available name
	// TODO: update R53 internal zone record to point to new staging DB
	// TODO: terminate old DB after done
	// TODO: implement lambda handler
	// TODO: error handling add returns - ex in entrypoint func - return errMsg, fmt.Errorf("LookupHost Err: %v", err)
	// TODO: deploy lambda
	// TODO: add monitoring if it fails to generate an alert
	awsRegion := "us-east-1"
	sourceDb := "schedool-db"
	destinationDb := "schedool-restore-db"

	// Init AWS Session and RDS Client
	rdsClient, initErr := initRDSClient(awsRegion)
	if initErr != nil {
		fmt.Errorf("Init Err: %w", initErr)
	}

	// Restore point int time RDS into a new cluster
	restoreErr := restorePointInTimeRDS(rdsClient, sourceDb, destinationDb)
	if restoreErr != nil {
		fmt.Errorf("Restore Err: %w", restoreErr)
	}
}

func initRDSClient(awsRegion string) (*rds.RDS, error) {
	// Create AWS session with default credentials and region (in ENV vars)
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion)},
	)
	if err != nil {
		return nil, fmt.Errorf("Initialize: Cannot create AWS config sessions: %w", err)
	}

	svc := rds.New(sess)
	return svc, nil
}

func restorePointInTimeRDS(rdsClientSess *rds.RDS, sourceDb string, destinationDb string) error {
	// Restore RDS cluster into a new cluster using point in time
	input := &rds.RestoreDBClusterToPointInTimeInput{
		DBClusterIdentifier:       aws.String(destinationDb),
		RestoreToTime:             parseTime(""),
		SourceDBClusterIdentifier: aws.String(sourceDb),
	}

	result, err := rdsClientSess.RestoreDBClusterToPointInTime(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBClusterAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBClusterAlreadyExistsFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeDBClusterNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeDBClusterQuotaExceededFault:
				fmt.Println(rds.ErrCodeDBClusterQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeDBClusterSnapshotNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterSnapshotNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeDBSubnetGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBSubnetGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInsufficientDBClusterCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientDBClusterCapacityFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInsufficientStorageClusterCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientStorageClusterCapacityFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInvalidDBClusterSnapshotStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterSnapshotStateFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInvalidDBClusterStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInvalidDBSnapshotStateFault:
				fmt.Println(rds.ErrCodeInvalidDBSnapshotStateFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInvalidRestoreFault:
				fmt.Println(rds.ErrCodeInvalidRestoreFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInvalidSubnet:
				fmt.Println(rds.ErrCodeInvalidSubnet, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeInvalidVPCNetworkStateFault:
				fmt.Println(rds.ErrCodeInvalidVPCNetworkStateFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeKMSKeyNotAccessibleFault:
				fmt.Println(rds.ErrCodeKMSKeyNotAccessibleFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeOptionGroupNotFoundFault:
				fmt.Println(rds.ErrCodeOptionGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeStorageQuotaExceededFault:
				fmt.Println(rds.ErrCodeStorageQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeDomainNotFoundFault:
				fmt.Println(rds.ErrCodeDomainNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			case rds.ErrCodeDBClusterParameterGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterParameterGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring DB")
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
			return fmt.Errorf("Error restoring DB")
		}
	}

	fmt.Println(result)
	return nil
}
