# Automated RDS Point-In-Time restore from one cluster to antoher 

## Vars
```
export awsRegion="us-east-1"

export sourceRDS="schedool-db"
export restoreRDS="schedool-restore-db"

export rdsSubnetGroup="rds-private-subnet"
export rdsSecurityGroupId="sg-04954e709e0dd8068"

export restoreDate="2021-08-21"
export restoreTime="21:00:00"
```
