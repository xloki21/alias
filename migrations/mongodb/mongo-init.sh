#!/bin/bash
echo "Creating users and roles..."
mongosh --eval "var rootUser = '$MONGO_INITDB_ROOT_USERNAME';
var rootPwd = '$MONGO_INITDB_ROOT_PASSWORD';
var admin = db.getSiblingDB('$MONGO_AUTHSOURCE');
admin.auth(rootUser, rootPwd);
admin.createUser({user: '$MONGO_USERNAME', pwd: '$MONGO_PASSWORD', roles: [{role: 'dbOwner', db: 'appdb'}]});
var appdb = db.getSiblingDB('appdb');

appdb.createCollection('aliases');
appdb.aliases.createIndex({'key': 1}, { unique: true });"
echo "Done!"