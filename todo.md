1. use iam instead of credentials
2. make a function to initialize bastionsubnetid instead of creating it. This will be needed by the bastion which should not create infra (oither than managing box pool)
3. review bastion code
4. five bastion a managed identity so it can manage the box pool
5. create a box, from the bastion

