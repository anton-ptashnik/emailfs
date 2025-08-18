# EmailFS

EmailFS - email in your Linux filesystem. EmailFS represents messages as files in a folder with the files named after email subjects, thus allowing one to use familiar Linux tools to list email messages and read content of simple text messages right within a terminal or a file manager.

## Initial setup

Currently the only supported email service is Gmail. 

For any app like EmailFS Gmail requires some pre-setup to access your mailbox. One needs to create a project in Google Console where app permissions are setup and then used to connect to Gmail. Follow the next steps to create a GC project:

1. Navigate to http://console.cloud.google.com
2. Create a new project

Project name: EmailFS or any
Organization: any, can be left empty

3. Select the newly create project
4. Select `APIs and Services` in Navigation menu, then `Enable APIs and services`
5. Search for `Gmail API` and enable it
6. Go to `Oauth Consent screen` and create it

App name: EmailFS or any
User support email: your email
Audience: external
Contact Information: your email

7. Go to `Clients`, create a new client

Application type: Desktop
Name: EmailFS

8. Once done, copy provided Client ID and Client Secret for future reference
9. Go to `Audience`, then publish the app
10. All done. Now go to the root dir with EmailFS exec, create `.env` and fill these values 

```
CLIENT_ID=
CLIENT_SECRET=
EMAIL_ADDRESS=
``` 

Once done you can start the app. Browser will be opened and you will be prompted to grant EmailFS access to your mailbox, once confirmed you'll have your emails listed within a folder.
