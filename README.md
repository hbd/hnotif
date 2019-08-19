# HNotif

Get notified so that you don't miss Hacker News headlines.

## Features

* Notifications for popular posts (# votes or comments)
* iOS and Apple Watch (email and Android TBD)

## Goals

#### _Notification service_:

1) Setup client for HN API.

2) Poll or consume events for story upvotes.

* Also do for # comments.

3) Notify subscribers if threshold is met.

(running service)

#### _Subscriber CRUD_:

*(create) Submit new subscription request*:

* email/push notif endpoints
* treshold for headline upvotes & # comments
* ex: if upvotes > 100 -> send notif
* (db) or (Lambda?)

*(retrieve?)*

*(update)*

* thresholds and endpoints

*(delete) Unsubscribe.*

#### _FE_:
* Webapp for email sub.
* Chrome notif?
* iOS for push notif.
* Apple Watch push notif.
* Android push notif.
