VOLUME_ID=`aws ec2 create-volume --availability-zone us-west-1b --volume-type gp2 --size 100 --query VolumeId --output text`
aws ec2 create-tags --resources ${VOLUME_ID} --tags Key=Name,Value=kafka-cluster1-1

echo ${VOLUME_ID}
