# Installing and Deploying Authorino

1. [Clone the repo](#clone-the-repo)
2. [Meet the requirements](#meet-the-requirements)
3. [Install and Deploy](#install-and-deploy)
   - [Option A: Local cluster](#option-a-local-cluster)
   - [Option B: Custom deployment](#option-b-custom-deployment)

## Clone the repo

This is the easiest way to get Authorino's Kubernetes resources and deployment utils.

```sh
git clone git@github.com:kuadrant/authorino.git && cd authorino
```

## Meet the requirements

Having a [Kubernetes](https://kubernetes.io/) server up and running is the only actual requirement to deploy Authorino.

With the tools provided in the Authorino repo nevertheless, you can easily have a local cluster setup (using [Kind](https://kind.sigs.k8s.io)). In this case, the only requirement is [Docker](https://docker.com).

Apart from the obvious requirement of having a running instance of a Kubernetes server, Authorino also relies on some other components and capabilities to perform its function:

<!-- TODO: Add minimum required Kubernetes version -->

- Permission from the Kubernetes server to create Custom Resource Definitions (CRDs) during Authorino's installation. Cluster administrators can handle this requirement through the Kubernetes API Role-Based Access Control bindings.
- [Envoy](https://www.envoyproxy.io) proxy (or, technically, any proxy that implements the client-side of the [gRPC protocol](https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/ext_authz)). With Envoy, ultimately, virtual hosts will be associated to the upstream APIs to protect, and the [external authorization filter](https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/ext_authz) should point to Authorino. Check [this example](/examples/envoy/envoy-deploy.yaml) if you need to.

### Feature-specific requirements

Some other feature-specific requirements (as opposed to actual requirements of Authorino) include:
- For OpenID Connect, make sure you have access to an identity provider (IdP) and an authority that can issue ID tokens (JWTs). You may want to check out [Keycloak](https://www.keycloak.org) which can solve both and connect to external identity sources and user federation like LDAP.
- For UMA-protected resource data, you will need a UMA-compliant server running as well. This can be an implementation of the UMA protocol by each upstream API itself or (more tipically) an external server that knows about the resources. Again, Keycloak can be a good fit here as well. Just keep in mind that, whatever resource server you choose, changing-state actions commanded in the upstream APIs or other parties will have to be reflected in the resource server. Authorino will not do that for you.
- For Kubernetes authentication tokens, in case you want to be able to requests access tokens for clients running outside the custer, you may want to check out the requisites for using Kubernetes [TokenRquest API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#tokenrequest-v1-storage-k8s-io) (GA in v1.20).

## Install and Deploy

Choose between the options below to continue:

[**Option A:** Local cluster](#option-a-local-cluster)<br/>
To try Authorino out and/or run the examples, based on a fresh image of Authorino built locally.<br/>
Setup may take up to 5 minutes.

[**Option B:** Custom deployment](#option-b-custom-deployment)<br/>
For deploying locally or in the cloud, with options to pick a pre-built image publicly available or build locally, define the name of your namespace, reconciliation mode, and number of replicas.<br/>
Setup time may vary from 2 to 10 minutes, depending on options chosen.

### Option A: Local cluster

Option A is meant for trying out Authorino locally. It gives you a bundle of a [Kind](https://kind.sigs.k8s.io)-managed Kubernetes cluster, with a freshly built Authorino image and pre-configured sample resources.

Included resources:<br/>
- **Talker API**<br/>
    Just a simple rack application that echoes back in a JSON whatever it gets in the request. You can control the response by passing the custom HTTP headers X-Echo-Status and X-Echo-Message (both optional).
- **Authorino**<br/>
    The Cloud-native AuthN/AuthZ enforcer that looks for `config.authorino.3scale.net/Service` custom resources in the Kubernetes server to add protection to your APIs.
- **Envoy proxy**<br/>
    Serving requests to the Talker API virtual host and configured with the ext_authz http filter pointing to the Authorino service.

To start the local Kubernetes cluster, build and deploy Authorino, run:

```sh
make local-setup
```

You can skip the local build of the image, and work with the default `quay.io/3scale/authorino:latest`, by using the `SKIP_LOCAL_BUILD` flag:

```sh
make local-setup SKIP_LOCAL_BUILD=1
```

After all deployments are ready and in case you want to consume protected services running inside the cluster from your local host, you can forward the requests on port 8000 to the Envoy service by running:

```sh
kubectl -n authorino port-forward deployment/envoy 8000:8000 &
```

Check out below the other options to start the local environment adding either [Keycloak](https://www.keycloak.org) or [Dex](https://dexidp.io) IAM servers to the bundle.

#### Deploy with Keycloak

The [Keycloak](https://www.keycloak.org) bundle included can be used to issue OIDC tokens for providing resource data for the authorization policies.

The bundle comes preloaded with the following sample configs:<br/>
- Admin console: http://localhost:8080/auth/admin (admin/p)
- Preloaded realm: **kuadrant**
- Preloaded clients:
  - **demo**: to which API consumers delegate access and therefore the one which access tokens are issued to
  - **authorino**: used by Authorino to fetch additional user info with `client_credentials` grant type
  - **talker-api**: used by Authorino to fetch UMA-protected resource data associated with the Talker API
- Preloaded resources:
  - `/hello`
  - `/greetings/1` (owned by user jonh)
  - `/greetings/2` (owned by user jane)
  - `/goodbye`
- Realm roles:
  - member (default to all users)
  - admin
- Preloaded users:
  - john/p (member)
  - jane/p (admin)
  - peter/p (member, email not verified)

```sh
make local-setup DEPLOY_KEYCLOAK=1
```

Forward local requests to the services running in the cluster, by running:

```sh
kubectl -n authorino port-forward deployment/envoy 8000:8000 &
kubectl -n authorino port-forward deployment/keycloak 8080:8080 &
```

#### Deploy with Dex

The [Dex](https://dexidp.io) bundle included is a simple OIDC identity provider server containing the following sample resources preloaded:<br/>
- Preloaded clients:<br/>
  - **demo**: to which API consumers delegate access and therefore the one which access tokens are issued to (Client secret: aaf88e0e-d41d-4325-a068-57c4b0d61d8e)
- Preloaded users:<br/>
  - marta@localhost/password

```sh
make local-setup DEPLOY_DEX=1
```

Forward local requests to the services running in the cluster, by running:

```sh
kubectl -n authorino port-forward deployment/envoy 8000:8000 &
kubectl -n authorino port-forward deployment/dex 5556:5556 &
```

#### Deploy with Keycloak and Dex

```sh
make local-setup DEPLOY_IDPS=1
```

```sh
kubectl -n authorino port-forward deployment/envoy 8000:8000 &
kubectl -n authorino port-forward deployment/keycloak 8080:8080 &
kubectl -n authorino port-forward deployment/dex 5556:5556 &
```

#### Clean up

Delete the local Kind-managed Kubernetes cluster, thus cleaning up all resources deployed:

```sh
make local-cleanup
```

### Option B: Custom deployment

The steps to custom deploy Authorino are divided in two parts: **installation** and **deployment**.

Installing Authorino refers to the step of applying the Authorino CRD and `ClusterRole`s to the Kubernetes cluster. This step requires admin privileges over the cluster and is performed only once per cluster.

Deploying Authorino instances refers to starting up Authorino external authorization service pods that will enforce auth configs on specified hosts. This step may or may not require admin privileges over the Kubernetes cluster, depending on the deployment mode that is chosen – i.e. **namespace-scoped** deployment or **cluster-wide** deployment.

In the end, a typical setup with one or more upstream APIs protected with Authorino and Envoy on a Kubernetes server, includes:<br/>
- i. Authorino definitions (CRD, `ClusterRole`s) and replica sets of the Authorino authorization service (`RoleBinding`s, `Deployment` and `Service`)
- ii. Envoy proxy (w/ ext_authz filter pointing to an instance of Authorino)
- iii. The one or more APIs ("upstreams") to be protected
- iv. Identity/authorization server (e.g. Keycloak), depending on the authentication methods of choice

The next steps provide some guidance on how to install and deploy Authorino, corresponding only to item (i) on the list of components above. To deploy and configure Envoy, as well as possibly required identity providers/authorization servers, please refer to the corresponding docs of each of those components.

The [examples](/examples) provided in the Authorino repo may as well offer some hints on how to finish the setup.

#### 1. Choose your server

The commands to install and deploy Authorino mostly assume you have a Kubernetes cluster where your `kubectl` is pointing at. Make sure your `kubectl` CLI is pointing to the Kubernetes cluster where you want to deploy Authorino.

In case you do not have a target Kubernetes server where to deploy Authorino yet, and simply want to try it out locally, you can launch a local cluster with [Kind](https://kind.sigs.k8s.io) by running:

```sh
make local-cluster-up
```

By defult, the name of the new local cluster will be "authorino". You can set a different one by changing Kind's context cluster name (environment variable `KIND_CLUSTER_NAME`).

#### 2. Install Authorino

To install Authorino Custom Resource Definition (CRD) and `ClusterRole`s, admins of the Kubernetes cluster can run:

```sh
make install
```

The command above will create the Authorino definitions in the cluster based on the manifests fetched with the code. It is imperative that this version of the manifests are compatible with the Authorino image chosen for the deployment in the next step.

#### 3. Choose an image

Chose or build an image of Authorino that is compatible with the version of the CRD installed in the previous step.

By default, `quay.io/3scale/authorino:latest` will be used. You can check out [quay.io/3scale/authorino](https://quay.io/3scale/authorino) for a list of pre-built image tags available.

If you choose to continue with the default Authorino image or any other publicly available pre-built image, you can go to the next step.

To build you own local image of Authorino from code, run:

```sh
make docker-build AUTHORINO_IMAGE=authorino:my-local-image
```

To push the image to a local Kubernetes cluster started with Kind, run:

```sh
make local-push AUTHORINO_IMAGE=authorino:my-local-image
```

In case you are not working with a local Kubernetes server started with `local-cluster-up`, but yet has built your own local image of Authorino, use normal `docker push` command to push the image to a registry of your preference.

#### 4. Create the namespace

To use the default name "authorino" for the namespace, run:

```sh
make namespace
```

You can change the name of the namespace by setting the `AUTHORINO_NAMESPACE` variable beforehand. In this case, it is recommended to export the variable to the shell, so the value is available as well for the next step, i.e. deploying Authorino.

```sh
export AUTHORINO_NAMESPACE="authorino"
make namespace
```

#### 5. Deploy Authorino instances

To deploy Auhorino instances, you can choose either **namespaced** instances or **cluster-wide** instances.

Namespace-scoped instances of Authorino only watch CRs and `Secret`s created in a given namespace. This deployment mode does not require admin privileges over the Kubernetes cluster to deploy.

Cluster-wide deployment mode, in contraposition, deploys instances of Authorino that watch CRs and `Secret` defined by users in any namespace across the cluster, consolidating all resources into one single cache of auth configs. Admin privileges over the Kubernetes cluster is required to deploy Authorino in cluster-wide reconciliation mode.

> **Warning:** It is NOT recommended to combine instances of Authorino deployed with both of this modes in the same Kubernetes cluster, but either only one or the other should be chosen for a given Kubernetes cluster at a time instead.

To deploy namespaced Authorino instances (`Deployment`, `Service` and `RoleBinding`s), run:

```sh
make deploy
```

or

```sh
make deploy AUTHORINO_DEPLOYMENT=namespaced
```

To deploy cluster-wide Authorino instances (`Deployment`, `Service` and `ClusterRoleBinding`s), run:

```sh
make deploy AUTHORINO_DEPLOYMENT=cluster-wide
```

##### TLS

By default, all deployments enable TLS on the endpoints served by Authorino (e.g. wristband/OIDC HTTP server).

If [cert-manager](https://cert-manager.io) CRDs are installed in the cluster and the `Secret`s required to enable TLS are not yet available in the namespace, `make deploy` will request TLS certificates to be issued by creating `Issuer` and `Certificate` cert-manager custom resources.

If you do not want to use cert-manager to manage Authorino TLS certificates, make sure to create the corresponding required `Secret` resources beforehand.

To completely disable TLS, append `-notls` to the value of the `AUTHORINO_DEPLOYMENT` parameter. In this case, neither cert-manager nor any TLS secrets are required, and Authorino will serve endpoints via `http` instead of `https`. E.g.:

```sh
make deploy AUTHORINO_DEPLOYMENT=namespaced-notls
```

or

```sh
make deploy AUTHORINO_DEPLOYMENT=cluster-wide-notls
```

##### Changing the image

By default, the commands above assume `quay.io/3scale/authorino:latest` to be the Authorino image tag to deploy. You can change that by setting the `AUTHORINO_IMAGE` parameter.

```sh
make deploy AUTHORINO_IMAGE=authorino:my-custom-image
```

> **NOTE:** In case you are working with a local Kubernetes cluster started with Kind, have built and pushed a local image to the server registry, remind of Kubernetes default pull policy, which establishes that the image tag `:latest` causes the policy `Always` to be enforced. In such case, you may want to change the policy to `IfNotPresent`. See [Kubernetes `imagePullPolicy`](https://kubernetes.io/docs/concepts/containers/images/#updating-images) for more information.

##### Number of replicas

You can tweak with the number of replicas of the Authorino `Deployment`, by setting the `AUTHORINO_REPLICAS` parameter. E.g.:

```sh
make deploy AUTHORINO_REPLICAS=4 AUTHORINO_DEPLOYMENT=namespaced AUTHORINO_IMAGE=quay.io/3scale/authorino:latest
```

#### Next steps

Finish the setup by deploying Envoy, upstream APIs to be protected with Authorino and possibly any required identity providers and authentication servers. You will then be ready to start creating `config.authorino.3scale.net/Service` custom resources representing the authN/authZ protection configs for your APIs.

Please check out as well the provided [examples](/examples) for more details about what can be done and the possible next steps to protect your APIs.
