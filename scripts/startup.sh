#!/bin/bash
set -e
set -x

if ! command -v java >/dev/null 2>&1
then
  echo "Installing Amazon Corretto 21..."
  wget -O - https://apt.corretto.aws/corretto.key | sudo gpg --dearmor -o /usr/share/keyrings/corretto-keyring.gpg && \
  echo "deb [signed-by=/usr/share/keyrings/corretto-keyring.gpg] https://apt.corretto.aws stable main" | sudo tee /etc/apt/sources.list.d/corretto.list
  sudo apt-get update; sudo apt-get install -y java-21-amazon-corretto-jdk
fi

mkdir -p /home/minecraft
mount /dev/disk/by-id/google-minecraft-data /home/minecraft
(crontab -l | grep -v -F "/home/minecraft/backup.sh" ; echo "0 */4 * * * /home/minecraft/backup.sh")| crontab -
cd /home/minecraft
screen -d -m -S mcs java -Xms256M -Xmx768M -jar server.jar nogui