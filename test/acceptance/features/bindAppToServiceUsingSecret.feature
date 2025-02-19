Feature: Bind values from a secret referred in backing service resource

    As a user I would like to inject into my app as env variables
    values persisted in a secret referred within service resource

    Background:
        Given Namespace [TEST_NAMESPACE] is used
        * Service Binding Operator is running

    Scenario: Inject into app a key from a secret referred within service resource
        Binding definition is declared on service CRD.
        Given Generic test application "ssa-1" is running
        And The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: backends.stable.example.com
                annotations:
                    service.binding/username: path={.status.data.dbCredentials},objectType=Secret,sourceKey=username
            spec:
                group: stable.example.com
                versions:
                  - name: v1
                    served: true
                    storage: true
                scope: Namespaced
                names:
                    plural: backends
                    singular: backend
                    kind: Backend
                    shortNames:
                      - bk
            """
        And The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: ssa-1-secret
            stringData:
                username: AzureDiamond
            """
        And The Custom Resource is present
            """
            apiVersion: stable.example.com/v1
            kind: Backend
            metadata:
                name: ssa-1-service
            spec:
                image: docker.io/postgres
                imageName: postgres
                dbName: db-demo
            status:
                data:
                    dbCredentials: ssa-1-secret
            """
        When Service Binding is applied
            """
            apiVersion: binding.operators.coreos.com/v1alpha1
            kind: ServiceBinding
            metadata:
                name: ssa-1
            spec:
                bindAsFiles: false
                services:
                  - group: stable.example.com
                    version: v1
                    kind: Backend
                    name: ssa-1-service
                application:
                    name: ssa-1
                    group: apps
                    version: v1
                    resource: deployments
            """
        Then Service Binding "ssa-1" is ready
        And The application env var "BACKEND_USERNAME" has value "AzureDiamond"

    Scenario: Inject into app all keys from a secret referred within service resource
        Given Generic test application "ssa-2" is running
        And The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: backends.stable.example.com
                annotations:
                    service.binding: path={.status.data.dbCredentials},objectType=Secret,elementType=map
            spec:
                group: stable.example.com
                versions:
                  - name: v1
                    served: true
                    storage: true
                scope: Namespaced
                names:
                    plural: backends
                    singular: backend
                    kind: Backend
                    shortNames:
                      - bk
            """
        And The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: ssa-2-secret
            stringData:
                username: AzureDiamond
                password: hunter2
            """
        And The Custom Resource is present
            """
            apiVersion: stable.example.com/v1
            kind: Backend
            metadata:
                name: ssa-2-service
            spec:
                image: docker.io/postgres
                imageName: postgres
                dbName: db-demo
            status:
                data:
                    dbCredentials: ssa-2-secret
            """
        When Service Binding is applied
            """
            apiVersion: binding.operators.coreos.com/v1alpha1
            kind: ServiceBinding
            metadata:
                name: ssa-2
            spec:
                bindAsFiles: false
                services:
                  - group: stable.example.com
                    version: v1
                    kind: Backend
                    name: ssa-2-service
                application:
                    name: ssa-2
                    group: apps
                    version: v1
                    resource: deployments
            """
        Then Service Binding "ssa-2" is ready
        And The application env var "BACKEND_USERNAME" has value "AzureDiamond"
        And The application env var "BACKEND_PASSWORD" has value "hunter2"

    @olm
    Scenario: Inject into app a key from a secret referred within service resource Binding definition is declared via OLM descriptor.

        Given Generic test application "ssd-1" is running
        * The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: backends.foo.example.com
            spec:
                group: foo.example.com
                versions:
                    - name: v1
                      served: true
                      storage: true
                scope: Namespaced
                names:
                    plural: backends
                    singular: backend
                    kind: Backend
                    shortNames:
                    - bs
            """
        And The Custom Resource Definition is present
            """
            apiVersion: operators.coreos.com/v1alpha1
            kind: ClusterServiceVersion
            metadata:
              annotations:
                capabilities: Basic Install
              name: backend-operator-foo.v0.1.0
            spec:
              customresourcedefinitions:
                owned:
                - description: Backend is the Schema for the backend API
                  kind: Backend
                  name: backends.foo.example.com
                  version: v1
                  specDescriptors:
                    - description: Host address
                      displayName: Host address
                      path: host
                      x-descriptors:
                        - service.binding:host
                  statusDescriptors:
                      - description: db credentials
                        displayName: db credentials
                        path: data.dbCredentials
                        x-descriptors:
                            - urn:alm:descriptor:io.kubernetes:Secret
                            - service.binding:username:sourceValue=username
              displayName: Backend Operator
              install:
                spec:
                  deployments:
                  - name: backend-operator
                    spec:
                      replicas: 1
                      selector:
                        matchLabels:
                          name: backend-operator
                      strategy: {}
                      template:
                        metadata:
                          labels:
                            name: backend-operator
                        spec:
                          containers:
                          - command:
                            - backend-operator
                            env:
                            - name: WATCH_NAMESPACE
                              valueFrom:
                                fieldRef:
                                  fieldPath: metadata.annotations['olm.targetNamespaces']
                            - name: POD_NAME
                              valueFrom:
                                fieldRef:
                                  fieldPath: metadata.name
                            - name: OPERATOR_NAME
                              value: backend-operator
                            image: REPLACE_IMAGE
                            imagePullPolicy: Always
                            name: backend-operator
                            resources: {}
                strategy: deployment
            """
        And The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: ssd-1-secret
            stringData:
                username: AzureDiamond
            """
        And The Custom Resource is present
            """
            apiVersion: foo.example.com/v1
            kind: Backend
            metadata:
                name: ssd-1-service
            spec:
                host: example.com
            status:
                data:
                    dbCredentials: ssd-1-secret
            """
        When Service Binding is applied
            """
            apiVersion: binding.operators.coreos.com/v1alpha1
            kind: ServiceBinding
            metadata:
                name: ssd-1
            spec:
                bindAsFiles: false
                services:
                  - group: foo.example.com
                    version: v1
                    kind: Backend
                    name: ssd-1-service
                application:
                    name: ssd-1
                    group: apps
                    version: v1
                    resource: deployments
            """
        Then Service Binding "ssd-1" is ready
        And The application env var "BACKEND_USERNAME" has value "AzureDiamond"
        And The application env var "BACKEND_HOST" has value "example.com"

    @olm
    Scenario: Inject into app all keys from a secret referred within service resource Binding definition is declared via OLM descriptor.

        Given Generic test application "ssd-2" is running
        * The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: backends.bar.example.com
            spec:
                group: bar.example.com
                versions:
                    - name: v1
                      served: true
                      storage: true
                scope: Namespaced
                names:
                    plural: backends
                    singular: backend
                    kind: Backend
                    shortNames:
                    - bs
            """
        And The Custom Resource is present
            """
            apiVersion: operators.coreos.com/v1alpha1
            kind: ClusterServiceVersion
            metadata:
              annotations:
                capabilities: Basic Install
              name: backend-operator-bar.v0.1.0
            spec:
              customresourcedefinitions:
                owned:
                - description: Backend is the Schema for the backend API
                  kind: Backend
                  name: backends.bar.example.com
                  version: v1
                  statusDescriptors:
                      - description: db credentials
                        displayName: db credentials
                        path: data.dbCredentials
                        x-descriptors:
                            - urn:alm:descriptor:io.kubernetes:Secret
                            - service.binding:elementType=map
              displayName: Backend Operator
              install:
                spec:
                  deployments:
                  - name: backend-operator
                    spec:
                      replicas: 1
                      selector:
                        matchLabels:
                          name: backend-operator
                      strategy: {}
                      template:
                        metadata:
                          labels:
                            name: backend-operator
                        spec:
                          containers:
                          - command:
                            - backend-operator
                            env:
                            - name: WATCH_NAMESPACE
                              valueFrom:
                                fieldRef:
                                  fieldPath: metadata.annotations['olm.targetNamespaces']
                            - name: POD_NAME
                              valueFrom:
                                fieldRef:
                                  fieldPath: metadata.name
                            - name: OPERATOR_NAME
                              value: backend-operator
                            image: REPLACE_IMAGE
                            imagePullPolicy: Always
                            name: backend-operator
                            resources: {}
                strategy: deployment
            """
        And The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: ssd-2-secret
            stringData:
                username: AzureDiamond
                password: hunter2
            """
        And The Custom Resource is present
            """
            apiVersion: bar.example.com/v1
            kind: Backend
            metadata:
                name: ssd-2-service
            spec:
                image: docker.io/postgres
                imageName: postgres
                dbName: db-demo
            status:
                data:
                    dbCredentials: ssd-2-secret
            """
        When Service Binding is applied
            """
            apiVersion: binding.operators.coreos.com/v1alpha1
            kind: ServiceBinding
            metadata:
                name: ssd-2
            spec:
                bindAsFiles: false
                services:
                  - group: bar.example.com
                    version: v1
                    kind: Backend
                    name: ssd-2-service
                application:
                    name: ssd-2
                    group: apps
                    version: v1
                    resource: deployments
            """
        Then Service Binding "ssd-2" is ready
        And The application env var "BACKEND_USERNAME" has value "AzureDiamond"
        And The application env var "BACKEND_PASSWORD" has value "hunter2"

    Scenario: Inject into app all keys from a secret existing in a same namespace with service and different from the service binding
        Given The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: backends.stable.example.com
                annotations:
                    service.binding: path={.status.credentials},objectType=Secret
            spec:
                group: stable.example.com
                versions:
                  - name: v1
                    served: true
                    storage: true
                scope: Namespaced
                names:
                    plural: backends
                    singular: backend
                    kind: Backend
                    shortNames:
                      - bk
            """
        * Namespace is present
            """
            apiVersion: v1
            kind: Namespace
            metadata:
                name: backend-services
            """
        * The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: ssa-3-secret
                namespace: backend-services
            stringData:
                username: AzureDiamond
                password: hunter2
            """
        * The Custom Resource is present
            """
            apiVersion: stable.example.com/v1
            kind: Backend
            metadata:
                name: ssa-3-service
                namespace: backend-services
            status:
                credentials: ssa-3-secret
            """
        * Generic test application "ssa-3" is running
        When Service Binding is applied
            """
            apiVersion: binding.operators.coreos.com/v1alpha1
            kind: ServiceBinding
            metadata:
                name: ssa-3
            spec:
                bindAsFiles: false
                services:
                  - group: stable.example.com
                    version: v1
                    kind: Backend
                    name: ssa-3-service
                    namespace: backend-services
                application:
                    name: ssa-3
                    group: apps
                    version: v1
                    resource: deployments
            """
        Then Service Binding "ssa-3" is ready
        And The application env var "BACKEND_USERNAME" has value "AzureDiamond"
        And The application env var "BACKEND_PASSWORD" has value "hunter2"

    # Remove this disable tag once this issue is closed: https://github.com/redhat-developer/service-binding-operator/issues/808
    @disabled
    Scenario: Inject data from secret referred at .spec.containers.envFrom.secretRef.name
        Given The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: db-secret-x
            stringData:
                username: AzureDiamond
                password: hunter2
            """
        * The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: backends.stable.example.com
                annotations:
                    service.binding: path={.spec.containers[0].envFrom[0].secretRef.name},objectType=Secret
            spec:
                group: stable.example.com
                versions:
                  - name: v1
                    served: true
                    storage: true
                scope: Namespaced
                names:
                    plural: backends
                    singular: backend
                    kind: Backend
                    shortNames:
                      - bk
            """
        * The Custom Resource is present
            """
            apiVersion: stable.example.com/v1
            kind: Backend
            metadata:
                name: backend-service-x
            spec:
                containers:
                - envFrom:
                  - secretRef:
                        name: db-secret-x
            """
        * Generic test application "myapp-x" is running
        When Service Binding is applied
          """
          apiVersion: binding.operators.coreos.com/v1alpha1
          kind: ServiceBinding
          metadata:
              name: sb-inject-secret-data
          spec:
              bindAsFiles: false
              services:
              - group: stable.example.com
                version: v1
                kind: Backend
                name: backend-service-x
              application:
                name: myapp-x
                group: apps
                version: v1
                resource: deployments
          """
        Then jq ".status.conditions[] | select(.type=="CollectionReady").status" of Service Binding "sb-inject-secret-data" should be changed to "True"
        And The application env var "BACKEND_USERNAME" has value "AzureDiamond"
        And The application env var "BACKEND_PASSWORD" has value "hunter2"

    Scenario: Bind application to provisioned service
        Given The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: provisioned-secret-1
            stringData:
                username: foo
                password: bar
            """
        * The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: provisionedbackends.stable.example.com
            spec:
                group: stable.example.com
                versions:
                  - name: v1
                    served: true
                    storage: true
                scope: Namespaced
                names:
                    plural: provisionedbackends
                    singular: provisionedbackend
                    kind: ProvisionedBackend
                    shortNames:
                      - pbk
            """
        * The Custom Resource is present
            """
            apiVersion: stable.example.com/v1
            kind: ProvisionedBackend
            metadata:
                name: provisioned-service-1
            spec:
                foo: bar
            status:
                binding:
                    name: provisioned-secret-1
            """
        * Generic test application "myaop-provision-srv" is running
        When Service Binding is applied
          """
          apiVersion: binding.operators.coreos.com/v1alpha1
          kind: ServiceBinding
          metadata:
              name: bind-provisioned-service-1
          spec:
              services:
              - group: stable.example.com
                version: v1
                kind: ProvisionedBackend
                name: provisioned-service-1
              application:
                name: myaop-provision-srv
                group: apps
                version: v1
                resource: deployments
          """
        Then Service Binding "bind-provisioned-service-1" is ready
        And jq ".status.secret" of Service Binding "bind-provisioned-service-1" should be changed to "provisioned-secret-1"
        And Content of file "/bindings/bind-provisioned-service-1/username" in application pod is
            """
            foo
            """
        And Content of file "/bindings/bind-provisioned-service-1/password" in application pod is
            """
            bar
            """

    Scenario: Fail binding to provisioned service if secret name is not provided
        Given The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: provisionedbackends.stable.example.com
                annotations:
                    "service.binding/provisioned-service": "true"
            spec:
                group: stable.example.com
                versions:
                  - name: v1
                    served: true
                    storage: true
                scope: Namespaced
                names:
                    plural: provisionedbackends
                    singular: provisionedbackend
                    kind: ProvisionedBackend
                    shortNames:
                      - pbk
            """
        * The Custom Resource is present
            """
            apiVersion: stable.example.com/v1
            kind: ProvisionedBackend
            metadata:
                name: provisioned-service-2
            spec:
                foo: bar
            """
        * Generic test application "myaop-provision-srv2" is running
        When Service Binding is applied
          """
          apiVersion: binding.operators.coreos.com/v1alpha1
          kind: ServiceBinding
          metadata:
              name: bind-provisioned-service-2
          spec:
              services:
              - group: stable.example.com
                version: v1
                kind: ProvisionedBackend
                name: provisioned-service-2
              application:
                name: myaop-provision-srv2
                group: apps
                version: v1
                resource: deployments
          """
        Then jq ".status.conditions[] | select(.type=="CollectionReady").status" of Service Binding "bind-provisioned-service-2" should be changed to "False"
        And jq ".status.conditions[] | select(.type=="Ready").status" of Service Binding "bind-provisioned-service-2" should be changed to "False"
        And jq ".status.conditions[] | select(.type=="CollectionReady").reason" of Service Binding "bind-provisioned-service-2" should be changed to "ErrorReadingBinding"

    Scenario: Bind application to provisioned service that has binding annotations as well
        Given The Secret is present
            """
            apiVersion: v1
            kind: Secret
            metadata:
                name: provisioned-secret-3
            stringData:
                username: foo
                password: bar
            """
        * The Custom Resource Definition is present
            """
            apiVersion: apiextensions.k8s.io/v1beta1
            kind: CustomResourceDefinition
            metadata:
                name: provisionedbackends.stable.example.com
            spec:
                group: stable.example.com
                versions:
                  - name: v1
                    served: true
                    storage: true
                scope: Namespaced
                names:
                    plural: provisionedbackends
                    singular: provisionedbackend
                    kind: ProvisionedBackend
                    shortNames:
                      - pbk
            """
        * The Custom Resource is present
            """
            apiVersion: stable.example.com/v1
            kind: ProvisionedBackend
            metadata:
                name: provisioned-service-3
                annotations:
                    "service.binding": "path={.spec.foo}"
            spec:
                foo: bla
            status:
                binding:
                    name: provisioned-secret-3
            """
        * Generic test application "myaop-provision-srv3" is running
        When Service Binding is applied
          """
          apiVersion: binding.operators.coreos.com/v1alpha1
          kind: ServiceBinding
          metadata:
              name: bind-provisioned-service-3
          spec:
              services:
              - group: stable.example.com
                version: v1
                kind: ProvisionedBackend
                name: provisioned-service-3
              application:
                name: myaop-provision-srv3
                group: apps
                version: v1
                resource: deployments
          """
        Then Service Binding "bind-provisioned-service-3" is ready
        And Content of file "/bindings/bind-provisioned-service-3/username" in application pod is
            """
            foo
            """
        And Content of file "/bindings/bind-provisioned-service-3/password" in application pod is
            """
            bar
            """
        And Content of file "/bindings/bind-provisioned-service-3/foo" in application pod is
            """
            bla
            """
