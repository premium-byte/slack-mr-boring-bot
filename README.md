## Required permissions

| OAuth Scope          | Description                                                                                |
|----------------------|--------------------------------------------------------------------------------------------|
| channels:history     | View messages and other content in public channels that Mr. Boring has been added to       |
| channels:read        | View basic information about public channels in a workspace                                |
| chat:write           | Send messages as Mr. Boring                                                                |
| chat:write.customize | Send messages as Mr. Boring with a customized username and avatar                          |
| chat:write.public    | Send messages to channels Mr. Boring isn't a member of                                     |
| commands             | Add shortcuts and/or slash commands that people can use                                    |
| groups:history       | View messages and other content in private channels that Mr. Boring has been added to      |
| groups:write         | Manage private channels that Mr. Boring has been added to and create new ones              |
| groups:write.topic   | Set the description of private channels                                                    |
| im:history           | View messages and other content in direct messages that Mr. Boring has been added to       |
| im:read              | View basic information about direct messages that Mr. Boring has been added to             |
| im:write             | Start direct messages with people                                                          |
| mpim:history         | View messages and other content in group direct messages that Mr. Boring has been added to |
| usergroups:read      | View user groups in a workspace                                                            |
| usergroups:write     | Create and manage user groups                                                              |
| users:read           | View people in a workspace                                                                 |
| users:read.email     | View email addresses of people in a workspace                                              |


## Commands
How to request a user replacement in a team or group
```
/replace pedro87silva in teams payments-zeus support
/replace pedro87silva in groups payments-zeus support
```

How to list the previous and current selected users in a team or group
```
/show selected teams payments-zeus support
```

How to list the available users in a team or group
```
/show available teams payments-zeus support  
```  

## Curl the Go server REST API (Test only)
```shell
curl -X POST http://localhost:9090/replace -d "command=@StarryNights99 in teams payments-zeus support" -d "
text=@StarryNights99 in teams payments-zeus support" | jq .
```

## Build docker image
```shell
docker build -t repo/slack-mr-boring-bot:1.0.6 . --progress=plain
```

## Publish docker image
```shell
docker push pedro87silva/slack-mr-boring-bot:1.0.6
```


# User Group permissions (Required to update user group users)

https://api.slack.com/methods/usergroups.update

![Screenshot 2024-08-01 at 01.17.19.png](..%2F..%2F..%2F..%2F..%2Fvar%2Ffolders%2Fgj%2Fsv22855d3r10jshk3f8tw__r0000gn%2FT%2FTemporaryItems%2FNSIRD_screencaptureui_z68wQu%2FScreenshot%202024-08-01%20at%2001.17.19.png)