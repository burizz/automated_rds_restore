package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
)

// TODO: test if replaced instance will be detected properly by Terraform and not try to replace it again
// TODO: bitbucket pipeline to build and push in ECR
// TODO: k8s cron job deploy - pulumi or helm
// TODO: implement instance Wait functions in the same way as the cluster ones
// TODO: loglevel = "DEBUG"
// TODO: delete all instances inside cluster, nevermind how many they are
// TODO: add monitoring if it fails to generate an alert

func main() {
	// Env Vars
	awsRegion := os.Getenv("awsRegion")

	sourceRDS := os.Getenv("sourceRDS")
	restoreRDS := os.Getenv("restoreRDS")
	rdsSubnetGroup := os.Getenv("rdsSubnetGroup")
	rdsSecurityGroupId := os.Getenv("rdsSecurityGroupId")

	// Optional restore date and time - defaults to latest available point in time
	restoreDate := os.Getenv("restoreDate")
	restoreTime := os.Getenv("restoreTime")

	// Optional instance type - defaults to db.t3.small
	rdsInstanceType := os.Getenv("rdsInstanceType")

	// Optional rds engine - defaults to aurora-mysql
	rdsEngine := os.Getenv("rdsEngine")

	// Params
	var restoreParams = map[string]string{
		"awsRegion": awsRegion,
		"sourceRDS": sourceRDS,
		"restoreRDS": restoreRDS,
		"rdsSubnetGroup": rdsSubnetGroup,
		"rdsSecurityGroupId": rdsSecurityGroupId,
	}

	// Init AWS Session and RDS Client
	rdsClient, initErr := initRDSClient(awsRegion)
	if initErr != nil {
		fmt.Printf("Init Err: %v", initErr)
		os.Exit(1)
	}

	// If date and time provided use it instead of last restorable time
	if restoreDate != "" {
		if restoreTime != "" {
			restoreParams["restoreFromTime"] = restoreDate + "T" + restoreTime + ".000Z"
		} else {
			restoreParams["restoreFromTime"] = restoreDate + "T01:00:00.000Z"
		}
		fmt.Printf("Restore time set to %v\n", restoreParams["restoreFromTime"])
	} else {
		fmt.Printf("Restore time set to latest available\n")
	}

	// If instance type provided change default
	if rdsInstanceType != "" {
		restoreParams["rdsInstanceType"] = rdsInstanceType
	} else {
		restoreParams["rdsInstanceType"] = "db.t3.small"
	}

	// If rds engine provided change default
	if rdsEngine != "" {
		restoreParams["rdsEngine"] = rdsEngine
	} else {
		restoreParams["rdsEngine"] = "aurora-mysql"
	}

	// Check if RDS instance exists, if it doesn't, skip Instance delete step
	rdsInstanceExists, checkRDSInstanceExistsErr := rdsInstanceExists(rdsClient, restoreParams)
	if checkRDSInstanceExistsErr != nil {
		fmt.Printf("Check if RDS Instance exists Err: %v", checkRDSInstanceExistsErr)
		os.Exit(1)
	}

	// Check if RDS instance exists, if it doesn't skip Instance delete step
	if rdsInstanceExists {
		// Delete RDS instance
		deleteInstanceErr := deleteRDSInstance(rdsClient, restoreParams)
		if deleteInstanceErr != nil {
			fmt.Printf("Delete RDS Instance Err: %v", deleteInstanceErr)
			os.Exit(1)
		}

		waitDeleteInstanceErr := waitUntilRDSInstanceDeleted(rdsClient, restoreParams)
		if waitDeleteInstanceErr != nil {
			fmt.Printf("Wait RDS Instance delete Err : %v", waitDeleteInstanceErr)
			os.Exit(1)
		}
	}

	// Check if RDS cluster exists, if it doesn't, skip Cluster delete step
	// Should be executed only if Instance is deleted first, as instance deletion actually deletes cluster as well
	rdsClusterExists, checkRDSClusterExistsErr := rdsClusterExists(rdsClient, restoreParams)
	if checkRDSClusterExistsErr != nil {
		fmt.Printf("Check if RDS Cluster exists Err: %v", checkRDSClusterExistsErr)
		os.Exit(1)
	}

	if rdsClusterExists {
		// Delete RDS cluster
		deleteClusterErr := deleteRDSCluster(rdsClient, restoreParams)
		if deleteClusterErr != nil {
			fmt.Printf("Delete RDS Cluster Err: %v", deleteClusterErr)
			os.Exit(1)
		}

		// Wait until RDS Cluster is deleted
		waitDeleteClusterErr := waitUntilRDSClusterDeleted(rdsClient, restoreParams)
		if waitDeleteClusterErr != nil {
			fmt.Printf("Wait RDS Cluster delete Err : %v", waitDeleteClusterErr)
			os.Exit(1)
		}
	}

	// Restore point in time RDS into a new cluster
	restoreErr := restorePointInTimeRDS(rdsClient, restoreParams)
	if restoreErr != nil {
		fmt.Printf("Restore Point-In-Time RDS Err: %v", restoreErr)
		os.Exit(1)
	}

	// Wait until DB instance created
	waitClusterCreateErr := waitUntilRDSClusterCreated(rdsClient, restoreParams)
	if waitClusterCreateErr != nil {
		fmt.Printf("Wait RDS Cluster create Err: %v", waitClusterCreateErr)
		os.Exit(1)
	}

	// Create RDS Instance in RDS Cluster
	createRDSInstanceErr := createRDSInstance(rdsClient, restoreParams)
	if createRDSInstanceErr != nil {
		fmt.Printf("Create RDS Instance Err: %v", createRDSInstanceErr)
		os.Exit(1)
	}

	// Wait until DB instance created in RDS cluster
	waitInstanceCreateErr := waitUntilRDSInstanceCreated(rdsClient, restoreParams)
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

func restorePointInTimeRDS(rdsClientSess *rds.RDS, restoreParams map[string]string) error {

	var input *rds.RestoreDBClusterToPointInTimeInput

	if restoreParams["restoreFromTime"] != "" {
		parsedTime, parseTimeErr := time.Parse(time.RFC3339, restoreParams["restoreFromTime"])
		if parseTimeErr != nil {
			return fmt.Errorf("Cannot Parse Time format: %v", parseTimeErr)
		}

		input = &rds.RestoreDBClusterToPointInTimeInput{
			DBClusterIdentifier:       aws.String(restoreParams["restoreRDS"]),		// Required
			UseLatestRestorableTime:   aws.Bool(false),								// Required
			RestoreToTime:			   aws.Time(parsedTime),						// Reqired if UseLatestRestorableTime is false
			DBSubnetGroupName:         aws.String(restoreParams["rdsSubnetGroup"]), // Not Required
			SourceDBClusterIdentifier: aws.String(restoreParams["sourceRDS"]),      // Required
			VpcSecurityGroupIds: []*string{
				aws.String(restoreParams["rdsSecurityGroupId"]),
			},
			Tags: []*rds.Tag{														// Not required
				{
					Key:   aws.String("ManagedBy"),
					Value: aws.String("Terraform"),
				},
			},
		}
	} else {
		input = &rds.RestoreDBClusterToPointInTimeInput{
			DBClusterIdentifier:       aws.String(restoreParams["restoreRDS"]),		// Required
			UseLatestRestorableTime:   aws.Bool(true),								// Required
			DBSubnetGroupName:         aws.String(restoreParams["rdsSubnetGroup"]), // Not Required
			SourceDBClusterIdentifier: aws.String(restoreParams["sourceRDS"]),      // Required
			VpcSecurityGroupIds: []*string{
				aws.String(restoreParams["rdsSecurityGroupId"]),
			},
			Tags: []*rds.Tag{														// Not required
				{
					Key:   aws.String("ManagedBy"),
					Value: aws.String("Terraform"),
				},
			},
		}
	}

	fmt.Printf("Creating RDS cluster [%v] from latest Point-In-Time restore of [%v]\n", restoreParams["restoreRDS"], restoreParams["sourceRDS"])

	_, err := rdsClientSess.RestoreDBClusterToPointInTime(input)
	errMsg := fmt.Sprintf("Error restoring RDS cluster [%v] -> [%v]", restoreParams["sourceRDS"], restoreParams["restoreRDS"])
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBClusterAlreadyExistsFault:
				fmt.Println(rds.ErrCodeDBClusterAlreadyExistsFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeDBClusterNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeDBClusterQuotaExceededFault:
				fmt.Println(rds.ErrCodeDBClusterQuotaExceededFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeDBClusterSnapshotNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterSnapshotNotFoundFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeDBSubnetGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBSubnetGroupNotFoundFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInsufficientDBClusterCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientDBClusterCapacityFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInsufficientStorageClusterCapacityFault:
				fmt.Println(rds.ErrCodeInsufficientStorageClusterCapacityFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInvalidDBClusterSnapshotStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterSnapshotStateFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInvalidDBClusterStateFault:
				fmt.Println(rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInvalidDBSnapshotStateFault:
				fmt.Println(rds.ErrCodeInvalidDBSnapshotStateFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInvalidRestoreFault:
				fmt.Println(rds.ErrCodeInvalidRestoreFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInvalidSubnet:
				fmt.Println(rds.ErrCodeInvalidSubnet, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeInvalidVPCNetworkStateFault:
				fmt.Println(rds.ErrCodeInvalidVPCNetworkStateFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeKMSKeyNotAccessibleFault:
				fmt.Println(rds.ErrCodeKMSKeyNotAccessibleFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeOptionGroupNotFoundFault:
				fmt.Println(rds.ErrCodeOptionGroupNotFoundFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeStorageQuotaExceededFault:
				fmt.Println(rds.ErrCodeStorageQuotaExceededFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeDomainNotFoundFault:
				fmt.Println(rds.ErrCodeDomainNotFoundFault, aerr.Error())
				return fmt.Errorf(errMsg)
			case rds.ErrCodeDBClusterParameterGroupNotFoundFault:
				fmt.Println(rds.ErrCodeDBClusterParameterGroupNotFoundFault, aerr.Error())
				return fmt.Errorf(errMsg)
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and Message from an error.
			fmt.Println(err.Error())
			return fmt.Errorf(errMsg)
		}
	}

	// TODO: DEBUG - fmt.Println(result)
	fmt.Printf("Executed RDS point-in-time restore for clusters [%v] -> [%v]\n", restoreParams["restoreRDS"], restoreParams["sourceRDS"])
	return nil
}

// Create RDS instance ine RDS cluster
func createRDSInstance(rdsClientSess *rds.RDS, restoreParams map[string]string) error {
	rdsClusterName := restoreParams["restoreRDS"]
	rdsInstanceName := restoreParams["restoreRDS"] + "-0" // TODO: this should be handled better

	input := &rds.CreateDBInstanceInput{
		DBClusterIdentifier:  aws.String(rdsClusterName),
		DBInstanceIdentifier: aws.String(rdsInstanceName),
		DBInstanceClass:      aws.String(restoreParams["rdsInstanceType"]),
		Engine:               aws.String(restoreParams["rdsEngine"]),
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
	return nil
}

// Delete RDS Intance in RDS Cluster
func deleteRDSInstance(rdsClientSess *rds.RDS, restoreParams map[string]string) error {
	rdsClusterName := restoreParams["restoreRDS"]
	rdsInstanceName := rdsClusterName + "-0" // TODO: this should be handled better

	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
		SkipFinalSnapshot:    aws.Bool(true),
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
func deleteRDSCluster(rdsClientSess *rds.RDS, restoreParams map[string]string) error {
	rdsClusterName := restoreParams["restoreRDS"]

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
func rdsInstanceExists(rdsClientSess *rds.RDS, restoreParams map[string]string) (dbExists bool, checkDBExistsErr error) {
	rdsClusterName := restoreParams["restoreRDS"]
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
func rdsClusterExists(rdsClientSess *rds.RDS, restoreParams map[string]string) (dbExists bool, checkDBExistsErr error) {
	rdsClusterName := restoreParams["restoreRDS"]

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
func waitUntilRDSClusterDeleted(rdsClientSess *rds.RDS, restoreParams map[string]string) error {

	rdsClusterName := restoreParams["restoreRDS"]

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
	fmt.Printf("Wait until RDS cluster [%v] is fully deleted...\n", rdsClusterName)

	start := time.Now()

	maxWaitAttempts := 120

	// Check until deleted
	for waitAttempt := 0; waitAttempt < maxWaitAttempts; waitAttempt++ {
		elapsedTime := time.Since(start).Seconds()

		if waitAttempt > 0 {
			formattedTime := strings.Split(fmt.Sprintf("%6v", elapsedTime), ".")
			fmt.Printf("Cluster deletion elapsed time: %vs\n", formattedTime[0])
		}

		resp, err := rdsClientSess.DescribeDBClusters(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				if aerr.Code() == rds.ErrCodeDBClusterNotFoundFault {
					fmt.Println("RDS Cluster deleted successfully")
					return nil
				} else {
					// Print the error, cast err to awserr.Error to get the Code and Message from an error.
					fmt.Println(err.Error())
					return fmt.Errorf("Wait RDS cluster deletion err %v", err)
				}
			}
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
func waitUntilRDSClusterCreated(rdsClientSess *rds.RDS, restoreParams map[string]string) error {
	rdsClusterName := restoreParams["restoreRDS"]

	maxWaitAttempts := 120

	input := &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(rdsClusterName),
	}

	fmt.Printf("Wait until RDS cluster [%v] is fully created ...\n", rdsClusterName)

	start := time.Now()

	// Check until created
	for waitAttempt := 0; waitAttempt < maxWaitAttempts; waitAttempt++ {
		elapsedTime := time.Since(start)
		if waitAttempt > 0 {
			formattedTime := strings.Split(fmt.Sprintf("%6v", elapsedTime), ".")
			fmt.Printf("Cluster creation elapsed time: %vs\n", formattedTime[0])
		}

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
func waitUntilRDSInstanceDeleted(rdsClientSess *rds.RDS, restoreParams map[string]string) error {
	rdsClusterName := restoreParams["restoreRDS"]
	rdsInstanceName := rdsClusterName + "-0" // TODO: this should be handled better

	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
	}

	// Check if RDS Instance exists
	_, describeErr := rdsClientSess.DescribeDBInstances(input)
	if describeErr != nil {
		if aerr, ok := describeErr.(awserr.Error); ok {
			if aerr.Code() == rds.ErrCodeDBInstanceNotFoundFault {
				fmt.Println(rds.ErrCodeDBInstanceNotFoundFault, aerr.Error())
				return fmt.Errorf("Wait RDS instance delete err %v", describeErr)
			} else {
				// Print the error, cast err to awserr.Error to get the Code and Message from an error.
				fmt.Println(describeErr.Error())
				return fmt.Errorf("Wait RDS instance delete err %v", describeErr)
			}
		}
	}

	start := time.Now()
	maxWaitAttempts := 120

	fmt.Printf("Wait until RDS instance [%v] in cluster [%v] is fully deleted...\n", rdsInstanceName, rdsClusterName)

	// Check until deleted
	for waitAttempt := 0; waitAttempt < maxWaitAttempts; waitAttempt++ {
		elapsedTime := time.Since(start)
		if waitAttempt > 0 {
			formattedTime := strings.Split(fmt.Sprintf("%6v", elapsedTime), ".")
			fmt.Printf("Instance deletion elapsed time: %vs\n", formattedTime[0])
		}

		resp, describeErr := rdsClientSess.DescribeDBInstances(input)

		if describeErr != nil {
			if aerr, ok := describeErr.(awserr.Error); ok {
				if aerr.Code() == rds.ErrCodeDBInstanceNotFoundFault {
					fmt.Println("RDS Instance deleted successfully")
					return nil
				} else {
					// Print the error, cast err to awserr.Error to get the Code and Message from an error.
					fmt.Println(describeErr.Error())
					return fmt.Errorf("Wait RDS instance delete err %v", describeErr)
				}
			}
		}

		fmt.Printf("Instance status: [%s]\n", *resp.DBInstances[0].DBInstanceStatus)
		// TODO: do i need to loop through this if more instances need to be deleted ? 
		if *resp.DBInstances[0].DBInstanceStatus== "terminated" {
			fmt.Printf("RDS instance [%v] deleted successfully\n", rdsClusterName)
			return nil
		}
		time.Sleep(30 * time.Second)

	}
	fmt.Printf("RDS instance [%v] deleted successfully in RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)
	return nil
}

// Wait until RDS instance in RDS Cluster is fully created
func waitUntilRDSInstanceCreated(rdsClientSess *rds.RDS, restoreParams map[string]string) error {
	rdsClusterName := restoreParams["restoreRDS"]
	rdsInstanceName := rdsClusterName + "-0" // TODO: this should be handled better

	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(rdsInstanceName),
	}

	start := time.Now()
	maxWaitAttempts := 120

	fmt.Printf("Wait until RDS instance [%v] in cluster [%v] is fully created ...\n", rdsInstanceName, rdsClusterName)

	for waitAttempt := 0; waitAttempt < maxWaitAttempts; waitAttempt++ {
		elapsedTime := time.Since(start)
		if waitAttempt > 0 {
			formattedTime := strings.Split(fmt.Sprintf("%6v", elapsedTime), ".")
			fmt.Printf("Instance creation elapsed time: %vs\n", formattedTime[0])
		}

		resp, err := rdsClientSess.DescribeDBInstances(input)

		if err != nil {
			return fmt.Errorf("Wait RDS instance create err %v", err)
		}

		fmt.Printf("Instance status: [%s]\n", *resp.DBInstances[0].DBInstanceStatus)
		if *resp.DBInstances[0].DBInstanceStatus== "available" {
			fmt.Printf("RDS instance [%v] created successfully\n", rdsClusterName)
			return nil
		}
		time.Sleep(30 * time.Second)
	}
	fmt.Printf("RDS instance [%v] created successfully in RDS cluster [%v]\n", rdsInstanceName, rdsClusterName)
	return nil
}

// Time formatting helper
func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%02dh%02dm", h, m)
}
