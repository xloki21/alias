#!/bin/bash
echo "Creating users and roles..."
mongosh --eval "var rootUser = '$MONGO_INITDB_ROOT_USERNAME';
var rootPwd = '$MONGO_INITDB_ROOT_PASSWORD';
var admin = db.getSiblingDB('$MONGO_AUTHSOURCE');
admin.auth(rootUser, rootPwd);
admin.createUser({user: '$MONGO_USERNAME', pwd: '$MONGO_PASSWORD', roles: [{role: 'dbOwner', db: 'appdb'},{role: 'dbOwner', db: 'appdb_test'}]});
var appdb = db.getSiblingDB('appdb');
var appdb_test = db.getSiblingDB('appdb_test');

appdb.createCollection('aliases');
appdb.aliases.createIndex({'alias': 1}, { unique: true });

appdb_test.createCollection('aliases');
appdb_test.aliases.createIndex({'alias': 1}, { unique: true });"
echo "Done!"