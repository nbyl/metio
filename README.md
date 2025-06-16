# Minecraft Setup

# Initial Setup

* Create bucket called `minecraft-byl-tofu-state`
* Deploy opentofu stuff:

```
# TODO: authenticate
tofu init
tofu apply -auto-approve
```

* Deploy Minecraft server according to [Guide](https://web.archive.org/web/20190528003754/https://cloud.google.com/solutions/gaming/minecraft-server#try_an_alternative_minecraft_server) with the following changes:
  * Install Java with [Coretto](https://docs.aws.amazon.com/corretto/latest/corretto-21-ug/generic-linux-install.html)



# Links

* https://web.archive.org/web/20190528003754/https://cloud.google.com/solutions/gaming/minecraft-server#try_an_alternative_minecraft_server
* https://cloud.google.com/blog/products/management-tools/brick-by-brick-learn-gcp-by-setting-up-a-minecraft-server?hl=en
