RESTSERVER=192.168.130.8

#생성시 자동 할당
vmID = 9f675bdd-848f-4eb4-9ef1-5584afe60346

curl -X GET http://$RESTSERVER:1024/controlvm/$vmID?connection_name=openstack-config01&action=suspend
