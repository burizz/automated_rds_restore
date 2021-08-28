# Automated RDS Point-In-Time restore from one cluster to antoher 

## Vars
```
export awsRegion="us-east-1"

export sourceRDS="test-db"
export restoreRDS="test-db-restore"

export rdsSubnetGroup="rds-private-subnet"
export rdsSecurityGroupId="sg-03254e409e0bd8218"

# optional restore date and time - defaults to latest available point in time
export restoreDate="2021-08-21"
export restoreTime="21:00:00"

# optional instance type - defaults to db.t3.small
export rdsInstanceType="db.t3.small"

# optional rds engine - defaults to aurora-mysql
export rdsEngine="aurora-mysql"
```
