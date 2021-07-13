terraform-http-backend
======================

Description
-----------

- Supports locking
- No account management needed
	- Any `POST` to a unique URL path + username + password combination will be saved as a new state
	- If that path + username + password combination already exists then it will be updated
	- This means that you can't setup multiple credentials for the same path
- View the current states by `curl`ing `/health/dump`


Example using Docker
--------------------

### Start the backend ###

```
# terraform-http-backend
; docker run -d -p 8080:8080 --name mc-thb markcaudill/terraform-http-backend:1.0.0
33bda94bdb2465c9608799dac19502ce13711e987393ceb6b409ec1911dc7a57

# terraform-http-backend
; docker logs -f mc-thb
2021/07/13 18:24:19 Listening on 0.0.0.0:8080
```

### Create `main.tf` ###

```hcl
terraform {
  backend "http" {
    lock_address = "http://localhost:8080/s/somepath"
    unlock_address = "http://localhost:8080/s/somepath"
    address = "http://localhost:8080/s/somepath"
  }
}

resource "null_resource" "test" {}
```

### Initialize Terraform ###

```
# terraform-http-backend
; tf init

Initializing the backend...

Successfully configured the backend "http"! Terraform will automatically
use this backend unless the backend configuration changes.
2021/07/13 14:34:51 [DEBUG] GET http://localhost:8080/s/somepath

Initializing provider plugins...
- Finding latest version of hashicorp/null...
- Installing hashicorp/null v3.1.0...
- Installed hashicorp/null v3.1.0 (signed by HashiCorp)

Terraform has created a lock file .terraform.lock.hcl to record the provider
selections it made above. Include this file in your version control repository
so that Terraform can guarantee to make the same selections by default when
you run "terraform init" in the future.

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

### Apply The Configuration ###

```
# terraform-http-backend
; tf apply
2021/07/13 14:35:15 [DEBUG] LOCK http://localhost:8080/s/somepath
2021/07/13 14:35:15 [DEBUG] GET http://localhost:8080/s/somepath

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the
following symbols:
  + create

Terraform will perform the following actions:

  # null_resource.test will be created
  + resource "null_resource" "test" {
      + id = (known after apply)
    }

Plan: 1 to add, 0 to change, 0 to destroy.

Do you want to perform these actions?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value: yes

null_resource.test: Creating...
null_resource.test: Creation complete after 0s [id=7800042524710214061]
2021/07/13 14:35:18 [DEBUG] GET http://localhost:8080/s/somepath
2021/07/13 14:35:18 [DEBUG] POST http://localhost:8080/s/somepath?ID=adff3699-fe30-26fd-dd63-e2a203543d87
2021/07/13 14:35:18 [DEBUG] UNLOCK http://localhost:8080/s/somepath

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.
```

### Verify The State Was Saved ###

The data and lock are `base64` encoded in the dump. This is kind of a bug as it wasn't intended so I'll most likely change it in a later version. But you can at least see that there is now data there.

If you run `terraform apply`, and then before you confirm with `yes`, you run `curl -L http://localhost:8080/health/dump` in another terminal, you should see some lock data. If, in another terminal in the same directory you run `terraform apply` while the first `terraform apply` is still running (and holding the lock), you should get an error about the state being locked.


```
# terraform-http-backend
; curl -L http://localhost:8080/health/dump
[{"Data":"ewogICJ2ZXJzaW9uIjogNCwKICAidGVycmFmb3JtX3ZlcnNpb24iOiAiMS4wLjIiLAogICJzZXJpYWwiOiAwLAogICJsaW5lYWdlIjogIjdlOGRlYzRhLWM1NWItMDgzYS1iOTRlLWIzYWY4ZmUzYjZmNSIsCiAgIm91dHB1dHMiOiB7fSwKICAicmVzb3VyY2VzIjogWwogICAgewogICAgICAibW9kZSI6ICJtYW5hZ2VkIiwKICAgICAgInR5cGUiOiAibnVsbF9yZXNvdXJjZSIsCiAgICAgICJuYW1lIjogInRlc3QiLAogICAgICAicHJvdmlkZXIiOiAicHJvdmlkZXJbXCJyZWdpc3RyeS50ZXJyYWZvcm0uaW8vaGFzaGljb3JwL251bGxcIl0iLAogICAgICAiaW5zdGFuY2VzIjogWwogICAgICAgIHsKICAgICAgICAgICJzY2hlbWFfdmVyc2lvbiI6IDAsCiAgICAgICAgICAiYXR0cmlidXRlcyI6IHsKICAgICAgICAgICAgImlkIjogIjc4MDAwNDI1MjQ3MTAyMTQwNjEiLAogICAgICAgICAgICAidHJpZ2dlcnMiOiBudWxsCiAgICAgICAgICB9LAogICAgICAgICAgInNlbnNpdGl2ZV9hdHRyaWJ1dGVzIjogW10sCiAgICAgICAgICAicHJpdmF0ZSI6ICJiblZzYkE9PSIKICAgICAgICB9CiAgICAgIF0KICAgIH0KICBdCn0K","Lock":""}]
```
