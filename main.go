package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
)

func main() {
	// TODO: implement lambda handler
	// TODO: error handling add returns - ex in entrypoint func
		// return errMsg, fmt.Errorf("LookupHost Err: %v", err)
	// TODO: deploy lambda // pulumi
	// TODO: add monitoring if it fails to generate an alert
	// TODO: test if replaced instance will be detected properly by Terraform and not try to replace it again
	awsRegion := "us-east-1"
	sourceRDS := "schedool-db"
	restoreRDS := "schedool-restore-db"
	// TODO: if restoreTime provided use it instead of UseLatestRestorableTime
	// restoreTime := ""
	// TODO: loglevel = "DEBUG"

	// Init AWS Session and RDS Client
	rdsClient, initErr := initRDSClient(awsRegion)
	if initErr != nil {
		fmt.Printf("Init Err: %v", initErr)
		os.Exit(1)
	}

	// Check if RDS instance exists, if it doesn't, skip Instance delete step
	rdsInstanceExists, checkRDSInstanceExistsErr := rdsInstanceExists(rdsClient, restoreRDS)
	if checkRDSInstanceExistsErr != nil {
		fmt.Printf("Check if RDS Instance exists Err: %v", checkRDSInstanceExistsErr)
		os.Exit(1)
	}

	if rdsInstanceExists {
		// Delete RDS instance
		deleteInstanceErr := deleteRDSInstance(rdsClient, restoreRDS)
		if deleteInstanceErr != nil {
			fmt.Printf("Delete RDS Instance Err: %v", deleteInstanceErr)
			os.Exit(1)
		}

		waitDeleteInstanceErr := waitUntilRDSInstanceDeleted(rdsClient, restoreRDS)
		if waitDeleteInstanceErr != nil {
			fmt.Printf("Wait RDS Instance delete Err : %v", waitDeleteInstanceErr)
			os.Exit(1)
		}
	}

	// Check if RDS cluster exists, if it doesn't, skip Cluster delete step
	rdsClusterExists, checkRDSClusterExistsErr := rdsClusterExists(rdsClient, restoreRDS)
	if checkRDSClusterExistsErr != nil {
		fmt.Printf("Check if RDS Cluster exists Err: %v", checkRDSClusterExistsErr)
		os.Exit(1)
	}

	if rdsClusterExists {
		// Delete RDS cluster
		deleteClusterErr := deleteRDSCluster(rdsClient, restoreRDS)
		if deleteClusterErr != nil {
			fmt.Printf("Delete RDS Cluster Err: %v", deleteClusterErr)
			os.Exit(1)
		}

		// Wait until RDS Cluster is deleted
		waitDeleteClusterErr := waitUntilRDSClusterDeleted(rdsClient, restoreRDS)
		if waitDeleteClusterErr != nil {
			fmt.Printf("Wait RDS Cluster delete Err : %v", waitDeleteClusterErr)
			os.Exit(1)
		}
	}

	// Restore point in time RDS into a new cluster
	restoreErr := restorePointInTimeRDS(rdsClient, sourceRDS, restoreRDS)
	if restoreErr != nil {
		fmt.Printf("Restore Point-In-Time RDS Err: %v", restoreErr)
		os.Exit(1)
	}

	// Wait until DB instance created
	waitClusterCreateErr := waitUntilRDSClusterCreated (rdsClient, restoreRDS)
	if waitClusterCreateErr != nil {
		fmt.Printf("Wait RDS Cluster create Err: %v", waitClusterCreateErr)
		os.Exit(1)
	}

	// Create RDS Instance in RDS Cluster
	createRDSInstanceErr := createRDSInstance(rdsClient, restoreRDS)
	if createRDSInstanceErr != nil {
		fmt.Printf("Create RDS Instance Err: %v", createRDSInstanceErr)
		os.Exit(1)
	}

	// Wait until DB instance created in RDS cluster
	waitInstanceCreateErr := waitUntilRDSInstanceCreated(rdsClient, restoreRDS)
	if waitInstanceCreateErr != nil {
		fmt.Printf("Wait RDS Instance create Err: %v", waitInstanceCreateErr)
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

	fmt.Printf("Creating RDS cluster [%v] from latest Point-In-Time restore of [%v]\n", destinationRDS, sourceRDS)

	_, err := rdsClientSess.RestoreDBClusterToPointInTime(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBClusterAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBClusterAlreadyExistsFault, aerr.Error())
				return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
			case rds.ErrCodeDBClusterNotFoundFault:
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
			// Print the error, cast err to awserr.Error to get the Code and Message from an error.
			fmt.Println(err.Error())
			return fmt.Errorf("Error restoring RDS cluster [%v] -> [%v]", sourceRDS, destinationRDS)
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("Executed RDS point-in-time restore for clusters [%v] -> [%v]\n", sourceRDS, destinationRDS)
	return nil
}

// Create RDS instance ine RDS cluster
func createRDSInstance(rdsClientSess *rds.RDS, rdsClusterName string) error {
	// TODO: figure out if there is a better way to handle any instance name or to just delete all instances inside cluster
	rdsInstanceName := rdsClusterName + "-0"
	input := &rds.CreateDBInstanceInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
		DBInstanceIdentifier: aws.String(rdsInstanceName),
		DBInstanceClass: aws.String("db.t3.small"),
		Engine: aws.String("aurora-mysql"),
	}

	fmt.Printf("Creating RDS Instance [%v] in RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)

	_, err := rdsClientSess.CreateDBInstance(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBInstanceAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBInstanceAlreadyExistsFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInsufficientDBInstanceCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientDBInstanceCapacityFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBParameterGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBParameterGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBSecurityGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBSecurityGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInstanceQuotaExceededFault:
				fmt.Println(rds.ErrCodeInstanceQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeStorageQuotaExceededFault:
				fmt.Println(rds.ErrCodeStorageQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBSubnetGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBSubnetGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBSubnetGroupDoesNotCoverEnoughAZs:
				fmt.Println(rds.ErrCodeDBSubnetGroupDoesNotCoverEnoughAZs, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidDBClusterStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidSubnet:
				fmt.Println(rds.ErrCodeInvalidSubnet, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidVPCNetworkStateFault:
				fmt.Println(rds.ErrCodeInvalidVPCNetworkStateFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeProvisionedIopsNotAvailableInAZFault:
				fmt.Println(rds.ErrCodeProvisionedIopsNotAvailableInAZFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeOptionGroupNotFoundFault:
				fmt.Println(rds.ErrCodeOptionGroupNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBClusterNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeStorageTypeNotSupportedFault:
				fmt.Println(rds.ErrCodeStorageTypeNotSupportedFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeAuthorizationNotFoundFault:
				fmt.Println(rds.ErrCodeAuthorizationNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeKMSKeyNotAccessibleFault:
				fmt.Println(rds.ErrCodeKMSKeyNotAccessibleFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDomainNotFoundFault:
				fmt.Println(rds.ErrCodeDomainNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeBackupPolicyNotFoundFault:
				fmt.Println(rds.ErrCodeBackupPolicyNotFoundFault, aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			default:
				fmt.Println(aerr.Error())
				return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and Message from an error.
			fmt.Println(err.Error())
			return fmt.Errorf("Error creating RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("Create RDS instance [%v]\n", rdsClusterName)
	return nil
}

// Delete RDS Intance in RDS Cluster
func deleteRDSInstance(rdsClientSess *rds.RDS, rdsClusterName string) error {
	// TODO: figure out if there is a better way to handle any instance name or to just delete all instances inside cluster
	rdsInstanceName := rdsClusterName + "-0"

	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
		SkipFinalSnapshot:   aws.Bool(true),
	}

	_, err := rdsClientSess.DeleteDBInstance(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBInstanceNotFoundFault:
				fmt.Println(rds.ErrCodeDBInstanceNotFoundFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidDBInstanceStateFault:
				fmt.Println(rds.ErrCodeInvalidDBInstanceStateFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBSnapshotAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBSnapshotAlreadyExistsFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeSnapshotQuotaExceededFault:
				fmt.Println(rds.ErrCodeSnapshotQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeInvalidDBClusterStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			case rds.ErrCodeDBInstanceAutomatedBackupQuotaExceededFault:
				fmt.Println(rds.ErrCodeDBInstanceAutomatedBackupQuotaExceededFault, aerr.Error())
				return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			default:
				fmt.Println(aerr.Error())
				return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
			return fmt.Errorf("Error deleting RDS instance [%v] in RDS cluster [%v]", rdsInstanceName, rdsClusterName)
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("Deleting RDS instance [%v] in RDS cluster [%v]\n", rdsClusterName, rdsClusterName)
	return nil
}

// Delete RDS Cluster
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
			return fmt.Errorf("Error deleting RDS cluster [%v]\n", rdsClusterName)
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("Deleting RDS cluster [%v]\n", rdsClusterName)
	return nil
}

// Check if RDS instance exists
func rdsInstanceExists(rdsClientSess *rds.RDS, rdsClusterName string) (dbExists bool, checkDBExistsErr error) {
	// TODO: figure out if there is a better way to handle any instance name or to just delete all instances inside cluster
	rdsInstanceName := rdsClusterName + "-0"

	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
	}

	_, err := rdsClientSess.DescribeDBInstances(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBInstanceNotFoundFault:
				// TODO: remove if not needed
				//fmt.Println(rds.ErrCodeDBInstanceNotFoundFault, aerr.Error())
				fmt.Printf("RDS instance [%v] doesnt exist, skipping delete step\n", rdsInstanceName)
				return false, nil
			default:
				fmt.Println(aerr.Error())
				return false, fmt.Errorf("Describe Err on cluster [%v]", rdsClusterName)
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and Message from an error.
			fmt.Println(err.Error())
			return false, fmt.Errorf("Describe Err on cluster [%v]", rdsClusterName)
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("RDS instance [%v] already exists, deleting it now ...\n", rdsInstanceName)
	return true, nil
}

// Check if RDS cluster exists
func rdsClusterExists(rdsClientSess *rds.RDS, rdsClusterName string) (dbExists bool, checkDBExistsErr error) {
	input := &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
	}

	_, err := rdsClientSess.DescribeDBClusters(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBClusterNotFoundFault:
				// TODO: remove if not needed
				//fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				fmt.Printf("RDS cluster [%v] doesnt exist, skipping delete step\n", rdsClusterName)
				return false, nil
			default:
				fmt.Println(aerr.Error())
				return false, fmt.Errorf("Describe Err on cluster [%v]", rdsClusterName)
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and Message from an error.
			fmt.Println(err.Error())
			return false, fmt.Errorf("Describe Err on cluster [%v]", rdsClusterName)
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("RDS cluster [%v] already exists, deleting it now ...\n", rdsClusterName)
	return true, nil
}

// Wait until RDS Cluster is fully deleted
func waitUntilRDSClusterDeleted(rdsClientSess *rds.RDS, rdsClusterName string) error {

	input := &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
	}


	// Check if Cluster exists
	_, err := rdsClientSess.DescribeDBClusters(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == rds.ErrCodeDBClusterNotFoundFault {
				fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				return fmt.Errorf("Wait RDS cluster deletion err %v", err)
			} else {
				// Print the error, cast err to awserr.Error to get the Code and Message from an error.
				fmt.Println(err.Error())
				return fmt.Errorf("Wait RDS cluster deletion err %v", err)
			}
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("Waiting until RDS cluster [%v] is fully deleted...\n", rdsClusterName)

	start := time.Now()

	maxWaitAttempts := 120

	// Check until deleted 
	for i := 0; i < maxWaitAttempts; i++ {
		// TODO: fix output sometimes appears as - 2.81e-07 
		elapsedTime := time.Since(start)
		fmt.Printf("Deletion elapsed time: %v\n", elapsedTime)

		resp, err := rdsClientSess.DescribeDBClusters(input)
		if err != nil {
				return fmt.Errorf("Wait RDS cluster deletion err %v", err)
		}

		fmt.Printf("Cluster status: [%s]\n", *resp.DBClusters[0].Status)
		if *resp.DBClusters[0].Status == "terminated" {
			fmt.Printf("RDS cluster [%v] deleted successfully\n", rdsClusterName)
			return nil
		}
		time.Sleep(30 * time.Second)
	}

	// Timeout Err
	return fmt.Errorf("RDS Cluster [%v] could not be deleted, exceed max wait attemps\n", rdsClusterName)
}

// Wait until RDS Cluster is fully created
func waitUntilRDSClusterCreated(rdsClientSess *rds.RDS, rdsClusterName string) error {
	maxWaitAttempts := 120

	input := &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
	}

	fmt.Printf("Waiting until RDS cluster [%v] is fully created ...\n", rdsClusterName)

	start := time.Now()

	for i := 0; i < maxWaitAttempts; i++ {
		// TODO: fix output sometimes appears as - 2.81e-07 
		elapsedTime := time.Since(start)
		fmt.Printf("Creation elapsed time: %v\n", elapsedTime)

		resp, err := rdsClientSess.DescribeDBClusters(input)

		if err != nil {
			return fmt.Errorf("Wait RDS cluster creation err %v", err)
		}

		fmt.Printf("Cluster status: [%s]\n", *resp.DBClusters[0].Status)
		if *resp.DBClusters[0].Status == "available" {
			fmt.Printf("RDS cluster [%v] created successfully\n", rdsClusterName)
			return nil
		}
		time.Sleep(30 * time.Second)
	}
	return fmt.Errorf("Aurora Cluster [%v] is not ready, exceed max wait attemps\n", rdsClusterName)
}


// Wait until RDS instance in RDS Cluster is fully created
func waitUntilRDSInstanceDeleted(rdsClientSess *rds.RDS, rdsClusterName string) error {
	// TODO: figure out if there is a better way to handle any instance name or to just delete all instances inside cluster
	rdsInstanceName := rdsClusterName + "-0"

	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
	}

	// Check if RDS Instance exists
	_, describeErr := rdsClientSess.DescribeDBInstances(input)
	if describeErr != nil {
		if aerr, ok := describeErr.(awserr.Error); ok {
			if aerr.Code() == rds.ErrCodeDBInstanceNotFoundFault {
				fmt.Println(rds.ErrCodeDBInstanceNotFoundFault, aerr.Error())
				return fmt.Errorf("Wait RDS instance deletion err %v", describeErr)
			} else {
				// Print the error, cast err to awserr.Error to get the Code and Message from an error.
				fmt.Println(describeErr.Error())
				return fmt.Errorf("Wait RDS instance deletion err %v", describeErr)
			}
		}
	}

	fmt.Printf("Waiting until RDS instance [%v] in cluster [%v] is fully deleted...\n", rdsInstanceName,  rdsClusterName)

	// Wait until RDS instance deleted
	waitErr := rdsClientSess.WaitUntilDBInstanceDeleted(input)
	if waitErr != nil {
		// TODO: remove this if not needed
		// return fmt.Errorf("Wait RDS instance deletion timeout err -  [%v]", waitErr)
		fmt.Printf("RDS instance [%v] deleted successfully in RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)
		return nil
	}

	fmt.Printf("RDS instance [%v] deleted successfully in RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)
	return nil
}

// Wait until RDS instance in RDS Cluster is fully created
func waitUntilRDSInstanceCreated(rdsClientSess *rds.RDS, rdsClusterName string) error {
	// TODO: figure out if there is a better way to handle any instance name or to just delete all instances inside cluster
	rdsInstanceName := rdsClusterName + "-0"

	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
	}

	fmt.Printf("Waiting until RDS instance [%v] in cluster [%v] is fully created ...\n", rdsInstanceName,  rdsClusterName)

	err := rdsClientSess.WaitUntilDBInstanceAvailable(input)
	if err != nil {
		return fmt.Errorf("Wait RDS instance creation timeout err [%v]", err)
	}

	fmt.Printf("RDS instance [%v] created successfully in RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)
	return nil
}
