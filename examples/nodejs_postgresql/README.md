# Binding an Imported app to an In-cluster Operator Managed PostgreSQL Database

## Introduction

This scenario illustrates binding an imported application to an in-cluster operated managed PostgreSQL Database.

Note that this example app is configured to operate with OpenShift 4.5 or newer.

## Actions to Perform by Users in 2 Roles

In this example there are 2 roles:

* Cluster Admin - Installs the operators to the cluster
* Application Developer - Imports a Node.js application, creates a DB instance, creates a request to bind the application and DB (to connect the DB and the application).

### Cluster Admin

The cluster admin needs to install 2 operators into the cluster:

* Service Binding Operator
* Backing Service Operator

A Backing Service Operator that is "bind-able," in other
words a Backing Service Operator that exposes binding information in secrets, config maps, status, and/or spec
attributes. The Backing Service Operator may represent a database or other services required by
applications. We'll use [postgresql-operator](https://github.com/operator-backing-service-samples/postgresql-operator) to
demonstrate a sample use case.

#### Install the Service Binding Operator

Navigate to the `Operators`->`OperatorHub` in the OpenShift console and in the `Developer Tools` category select the `Service Binding Operator` operator

![Service Binding Operator as shown in OperatorHub](../../assets/operator-hub-sbo-screenshot.png)

and install the `beta` version.

This makes the `ServiceBinding` custom resource available, that the application developer will use later.

#### Install the DB operator using a `CatalogSource`

Apply the following `CatalogSource`:

```shell
kubectl apply -f - << EOD
---
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
    name: sample-db-operators
    namespace: openshift-marketplace
spec:
    sourceType: grpc
    image: quay.io/redhat-developer/sample-db-operators-olm:v1
    displayName: Sample DB Operators
EOD
```

Then navigate to the `Operators`->`OperatorHub` in the OpenShift console and in the `Database` category select the `PostgreSQL Database` operator

![PostgreSQL Database Operator as shown in OperatorHub](../../assets/operator-hub-pgo-screenshot.png)

and install a `beta` version.

This makes the `Database` custom resource available, that the application developer will use later.

### Application Developer

#### Create a namespace called `service-binding-demo`

The application and the DB needs a namespace to live in so let's create one for them:

```shell
kubectl create namespace service-binding-demo
```

#### Import an application

In this example we will import an arbitrary [Node.js application](https://github.com/pmacik/nodejs-rest-http-crud).

In the OpenShift Console switch to the Developer perspective. (Make sure you have selected the `service-binding-demo` project). Navigate to the `+Add` page from the menu and then click on the `[From Git]` button. Fill in the form with the following:

* `Project` = `service-binding-demo`
* `Git Repo URL` = `https://github.com/pmacik/nodejs-rest-http-crud`
* `Builder Image` = `Node.js`
* `Application Name` = `nodejs-app`
* `Name` = `nodejs-app`

* `Select the resource type to generate` = Deployment
* `Create a route to the application` = checked

and click on the `[Create]` button.

Notice, that during the import no DB config was mentioned or requested.

When the application is running navigate to its route to verify that it is up. Notice that in the header it says `(DB: N/A)`. That means that the application is not connected to a DB and so it should not work properly. Try the application's UI to add a fruit - it causes an error proving that the DB is not connected.

#### Create a DB instance for the application

Now we utilize the DB operator that the cluster admin has installed. To create a DB instance just create a `Database` custom resource in the `service-binding-demo` namespace called `db-demo`:

```shell
kubectl apply -f - << EOD
---
apiVersion: postgresql.baiju.dev/v1alpha1
kind: Database
metadata:
  name: db-demo
  namespace: service-binding-demo
spec:
  image: docker.io/postgres
  imageName: postgres
  dbName: db-demo
EOD
```

#### Express an intent to bind the DB and the application

Now, the only thing that remains is to connect the DB and the application. We let the Service Binding Operator to 'magically' do the connection for us.

Create the following `ServiceBinding`:

```shell
kubectl apply -f - << EOD
---
apiVersion: binding.operators.coreos.com/v1alpha1
kind: ServiceBinding
metadata:
  name: binding-request
  namespace: service-binding-demo
spec:
  bindAsFiles: false
  application:
    name: nodejs-app
    group: apps
    version: v1
    resource: deployments
  services:
  - group: postgresql.baiju.dev
    version: v1alpha1
    kind: Database
    name: db-demo
  mappings:
    - name: DATABASE_DBCONNECTIONIP
      value: "{{ .postgresDB.status.dbConnectionIP }}"
    - name: DATABASE_DBCONNECTIONPORT
      value: "{{ .postgresDB.status.dbConnectionPort }}"
    - name: DATABASE_SECRET_USER
#     value: "{{ .postgresDB.status.dbCredentials.user }}",
      value: "postgres"
    - name: DATABASE_SECRET_PASSWORD
#     value: "{{ .postgresDB.status.dbCredentials.password }}"
      value: "password"
    - name: DATABASE_DBNAME
      value: "{{ .postgresDB.status.dbName }}"
EOD
```
Note: The issue https://github.com/redhat-developer/service-binding-operator/issues/982 is opened for tracking user and password problem.


* `application` - used to search for the application based on the name that we set earlier and the `group`, `version` and `resource` of the application to be a `Deployment`.
* `services` - used to find the backing service - our operator-backed DB instance called `db-demo`.
* `mappings` - used to inject the custom environment variables to application deployment, it's optional.


That causes the application to be re-deployed.

Once the new version is up, go to the application's route to check the UI. In the header you can see `(DB: db-demo)` which indicates that the application is connected to a DB and its name is `db-demo`. Now you can try the UI again but now it works!

When the `ServiceBinding` was created the Service Binding Operator's controller injected the DB connection information into the application's `Deployment` as environment variables via an intermediate `Secret` called `binding-request`， follows as example:


##### If bindAsFiles: false, the envFrom section will be injected into application deployment
``` yaml
spec:
  template:
    spec:
      containers:
      - envFrom:
        - secretRef:
            name: binding-request-44ddf789
```
Check the DB connection information as environment variables in application pod:
```console
# env | grep DATA
DATABASE_SECRET_USER=postgres
DATABASE_SECRET_PASSWORD=password
DATABASE_DBNAME=db-demo
DATABASE_USER=postgres
DATABASE_DB.USER=postgres
DATABASE_DB.NAME=db-demo
DATABASE_DB.PORT=5432
DATABASE_DBCONNECTIONPORT=5432
DATABASE_IMAGENAME=postgres
DATABASE_DB.HOST=172.30.58.28
DATABASE_PASSWORD=password
DATABASE_DB.PASSWORD=password
DATABASE_DBCONNECTIONIP=172.30.58.28
DATABASE_IMAGE=docker.io/postgres
```

##### If bindAsFiles: true, the secret will be mounted as volume in application pod.
``` yaml
spec:
  template:
    spec:
      containers:
      - env:
        volumeMounts:
        - mountPath: /bindings/binding-request
          name: binding-request
      volumes:
      - name: binding-request
        secret:
          defaultMode: 420
          secretName: binding-request-44ddf789
```
Check the DB connection information as files in application pod:
```shell
oc exec -it nodejs-app-869bb569d-cvwqw bash
bash-4.2$ cd bindings/binding-request
bash-4.2$ ls
DATABASE_DBCONNECTIONIP DATABASE_DBCONNECTIONPORT DATABASE_DBNAME  DATABASE_SECRET_PASSWORD DATABASE_SECRET_USER  
db.host  db.name  db.password  db.port  db.user  dbConnectionIP  dbConnectionPort  dbName  image  imageName  password  user
bash-4.2$ cat DATABASE_DBCONNECTIONIP; echo
172.30.58.28
bash-4.2$ cat DATABASE_DBCONNECTIONPORT; echo
5432
bash-4.2$ cat db.user; echo
postgres
bash-4.2$ cat db.password; echo
password
```

#### Check the status of Service Binding

`ServiceBinding Status` depicts the status of the Service Binding operator. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status

To check the status of Service Binding, run the command:

```
kubectl get servicebinding binding-request -n service-binding-demo -o yaml
```

Status of Service Binding on successful binding:

```yaml
status:
  conditions:
  - lastHeartbeatTime: "2020-10-15T13:23:36Z"
    lastTransitionTime: "2020-10-15T13:23:23Z"
    status: "True"
    type: CollectionReady
  - lastHeartbeatTime: "2020-10-15T13:23:36Z"
    lastTransitionTime: "2020-10-15T13:23:23Z"
    status: "True"
    type: InjectionReady
  secret: binding-request-72ddc0c540ab3a290e138726940591debf14c581
```

where

* Conditions represent the latest available observations of Service Binding's state
* Secret represents the name of the secret created by the Service Binding Operator


Conditions have two types `CollectionReady` and `InjectionReady`

where

* `CollectionReady` type represents collection of secret from the service
* `InjectionReady` type represents an injection of the secret into the application

Conditions can have the following type, status and reason:

| Type            | Status | Reason               | Type           | Status | Reason                   |
| --------------- | ------ | -------------------- | -------------- | ------ | ------------------------ |
| CollectionReady | False  | EmptyServiceSelector | InjectionReady | False  |                          |
| CollectionReady | False  | ServiceNotFound      | InjectionReady | False  |                          |
| CollectionReady | True   |                      | InjectionReady | False  | EmptyApplicationSelector |
| CollectionReady | True   |                      | InjectionReady | False  | ApplicationNotFound      |
| CollectionReady | True   |                      | InjectionReady | True   |                          |

That's it, folks!
