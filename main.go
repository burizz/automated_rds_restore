package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
)

func main() {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/rds/#RDS.RestoreDBClusterToPointInTime
	// TODO: implement lambda handler
	// TODO: error handling add returns - ex in entrypoint func - return errMsg, fmt.Errorf("LookupHost Err: %v", err)
	// TODO: deploy lambda
	// TODO: add monitoring if it fails to generate an alert
	// TODO: test if replaced instance will be detected properly by Terraform and not try to replace it again
	awsRegion := "us-east-1"
	sourceRDS := "schedool-db"
	restoreRDS := "schedool-restore-db"

	// Init AWS Session and RDS Client
	rdsClient, initErr := initRDSClient(awsRegion)
	if initErr != nil {
		fmt.Printf("Init Err: %v", initErr)
		os.Exit(1)
	}

	// Check if RDS cluster exists, if it does, skip deletion step
	rdsClusterExists, checkDBExistsErr := rdsClusterExists(rdsClient, restoreRDS)
	if checkDBExistsErr != nil {
		fmt.Printf("Check if RDS exists Err: %v", checkDBExistsErr)
	}

	if rdsClusterExists {
		// Delete RDS cluster
		deleteErr := deleteRDSCluster(rdsClient, restoreRDS)
		if deleteErr != nil {
			fmt.Printf("Delete RDS Err: %v", deleteErr)
			os.Exit(1)
		}

		// Wait until RDS Cluster is deleted
		waitDeleteErr := waitUntilRDSClusterDeleted(rdsClient, restoreRDS)
		if waitDeleteErr != nil {
			fmt.Printf("Wait RDS delete Err : %v", waitDeleteErr)
		}
	}

	// Restore point in time RDS into a new cluster
	restoreErr := restorePointInTimeRDS(rdsClient, sourceRDS, restoreRDS)
	if restoreErr != nil {
		fmt.Printf("Restore RDS Err: %v", restoreErr)
		os.Exit(1)
	}

	// Wait until DB instance created
	waitCreateErr := waitUntilRDSClusterCreated (rdsClient, restoreRDS)
	if waitCreateErr != nil {
		fmt.Printf("Wait RDS create Err: %v", waitCreateErr)
		os.Exit(1)
	}

	createRDSInstanceErr := createRDSInstance(rdsClient, restoreRDS)
	if createRDSInstanceErr != nil {
		fmt.Printf("Create RDS Instance Err: %v", createRDSInstanceErr)
		os.Exit(1)
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
	fmt.Printf("AWS RDS Client initialized successfully\n")
	return svc, nil
}

func restorePointInTimeRDS(rdsClientSess *rds.RDS, sourceRDS string, destinationRDS string) error {
	// Restore RDS cluster into a new cluster using point in time
	input := &rds.RestoreDBClusterToPointInTimeInput{
		DBClusterIdentifier:       aws.String(destinationRDS),
		UseLatestRestorableTime:   aws.Bool(true),
		SourceDBClusterIdentifier: aws.String(sourceRDS),
	}

	_, err := rdsClientSess.RestoreDBClusterToPointInTime(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBClusterAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBClusterAlreadyExistsFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeDBClusterNotFoundFault:
				fmt.Println("HERE")
				fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeDBClusterQuotaExceededFault:
				fmt.Println(rds.ErrCodeDBClusterQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeDBClusterSnapshotNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterSnapshotNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeDBSubnetGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBSubnetGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInsufficientDBClusterCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientDBClusterCapacityFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInsufficientStorageClusterCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientStorageClusterCapacityFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInvalidDBClusterSnapshotStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterSnapshotStateFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInvalidDBClusterStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInvalidDBSnapshotStateFault:
				fmt.Println(rds.ErrCodeInvalidDBSnapshotStateFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInvalidRestoreFault:
				fmt.Println(rds.ErrCodeInvalidRestoreFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInvalidSubnet:
				fmt.Println(rds.ErrCodeInvalidSubnet, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeInvalidVPCNetworkStateFault:
				fmt.Println(rds.ErrCodeInvalidVPCNetworkStateFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeKMSKeyNotAccessibleFault:
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeOptionGroupNotFoundFault:
				fmt.Println(rds.ErrCodeOptionGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeStorageQuotaExceededFault:
				fmt.Println(rds.ErrCodeStorageQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeDomainNotFoundFault:
				fmt.Println(rds.ErrCodeDomainNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeDBClusterParameterGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterParameterGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
			return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
		}
	}

	fmt.Printf("Executed RDS point-in-time restore for clusters [%v] -> [%v]\n", sourceRDS, destinationRDS)
	return nil
}

func createRDSInstance(rdsClientSess *rds.RDS, rdsClusterName string) error {
	rdsInstanceName := rdsClusterName + "-0"
	input := &rds.CreateDBInstanceInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
		DBInstanceIdentifier: aws.String(rdsInstanceName),
		DBInstanceClass: aws.String("db.t3.small"),
	}

	fmt.Printf("Creating RDS Instance [%v] inside RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)

	result, err := rdsClientSess.CreateDBInstance(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBInstanceAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBInstanceAlreadyExistsFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInsufficientDBInstanceCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientDBInstanceCapacityFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBParameterGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBParameterGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBSecurityGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBSecurityGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInstanceQuotaExceededFault:
				fmt.Println(rds.ErrCodeInstanceQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeStorageQuotaExceededFault:
				fmt.Println(rds.ErrCodeStorageQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBSubnetGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBSubnetGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBSubnetGroupDoesNotCoverEnoughAZs:
				fmt.Println(rds.ErrCodeDBSubnetGroupDoesNotCoverEnoughAZs, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidDBClusterStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidSubnet:
				fmt.Println(rds.ErrCodeInvalidSubnet, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidVPCNetworkStateFault:
				fmt.Println(rds.ErrCodeInvalidVPCNetworkStateFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeProvisionedIopsNotAvailableInAZFault:
				fmt.Println(rds.ErrCodeProvisionedIopsNotAvailableInAZFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeOptionGroupNotFoundFault:
				fmt.Println(rds.ErrCodeOptionGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBClusterNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeStorageTypeNotSupportedFault:
				fmt.Println(rds.ErrCodeStorageTypeNotSupportedFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeAuthorizationNotFoundFault:
				fmt.Println(rds.ErrCodeAuthorizationNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeKMSKeyNotAccessibleFault:
				fmt.Println(rds.ErrCodeKMSKeyNotAccessibleFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDomainNotFoundFault:
				fmt.Println(rds.ErrCodeDomainNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeBackupPolicyNotFoundFault:
				fmt.Println(rds.ErrCodeBackupPolicyNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			default:
				fmt.Println(aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
			return fmt.Errorf("Error creating RDS instance [%v] inside RDS cluster [%v]", rdsInstanceName, rdsClusterName)
		}
	}

	// TODO: remove once tested
	fmt.Println(result)
	return nil
}

func deleteRDSCluster(rdsClientSess *rds.RDS, rdsClusterName string) error {
	input := &rds.DeleteDBClusterInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
		SkipFinalSnapshot:   aws.Bool(true),
	}

	_, err := rdsClientSess.DeleteDBCluster(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBClusterNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS cluster [%v]\n", rdsClusterName)
			case rds.ErrCodeInvalidDBClusterStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS cluster [%v]\n", rdsClusterName)
			case rds.ErrCodeDBClusterSnapshotAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBClusterSnapshotAlreadyExistsFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS cluster [%v]\n", rdsClusterName)
			case rds.ErrCodeSnapshotQuotaExceededFault:
				fmt.Println(rds.ErrCodeSnapshotQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS cluster [%v]\n", rdsClusterName)
			case rds.ErrCodeInvalidDBClusterSnapshotStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterSnapshotStateFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS cluster [%v]\n", rdsClusterName)
			default:
				fmt.Println(aerr.Error())
				return fmt.Errorf("Error deleting RDS cluster [%v]\n", rdsClusterName)
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
	}

	fmt.Printf("Deleting RDS cluster [%v]\n", rdsClusterName)
	return nil
}

func rdsClusterExists(rdsClientSess *rds.RDS, rdsClusterName string) (dbExists bool, checkDBExistsErr error) {
	input := &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
	}

	_, err := rdsClientSess.DescribeDBClusters(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBClusterNotFoundFault:
				//fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				fmt.Printf("RDS cluster [%v] doesnt exist, skipping delete step", rdsClusterName)
				return false, nil
			default:
				fmt.Println(aerr.Error())
				return false, fmt.Errorf("Describe Err on cluster [%v]\n", rdsClusterName)
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
				return false, fmt.Errorf("Describe Err on cluster [%v]\n", rdsClusterName)
		}
	}

	fmt.Printf("RDS cluster [%v] already exists, deleting it now ...", rdsClusterName)
	return true, nil
}

func waitUntilRDSClusterDeleted(rdsClientSess *rds.RDS, rdsClusterName string) error {
	input := &rds.DescribeDBInstancesInput{
		Filters: []*rds.Filter{
			{
				Name: aws.String(rdsClusterName),
			},
		},

	}

	fmt.Printf("Waiting until RDS cluster [%v] is fully deleted ...", rdsClusterName)

	err := rdsClientSess.WaitUntilDBInstanceDeleted(input)
	if err != nil {
		return fmt.Errorf("Wait RDS cluster deletion timeout err %v", err)
	}

	fmt.Printf("RDS cluster [%v] deleted successfully\n", rdsClusterName)
	return nil
}

func waitUntilRDSClusterCreated(rdsClientSess *rds.RDS, rdsClusterName string) error {
	input := &rds.DescribeDBInstancesInput{
		Filters: []*rds.Filter{
			{
				Name: aws.String(rdsClusterName),
			},
		},
	}

	fmt.Printf("Waiting until RDS cluster [%v] is fully created ...", rdsClusterName)

	err := rdsClientSess.WaitUntilDBInstanceAvailable(input)
	if err != nil {
		return fmt.Errorf("Wait RDS cluster creation timeout err %v", err)
	}

	fmt.Printf("RDS cluster [%v] created successfully\n", rdsClusterName)
	return nil
}

func waitUntilRDSInstanceCreated(rdsClientSess *rds.RDS, rdsClusterName string) error {
	rdsInstanceName := rdsClusterName + "-0"

	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
	}

	fmt.Printf("Waiting until RDS instance [%v] inside cluster [%v] is fully created ...", rdsInstanceName,  rdsClusterName)

	err := rdsClientSess.WaitUntilDBInstanceAvailable(input)
	if err != nil {
		return fmt.Errorf("Wait RDS instance creation timeout err [%v]", err)
	}

	fmt.Printf("RDS instance [%v] created successfully inside RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)
	return nil
}
