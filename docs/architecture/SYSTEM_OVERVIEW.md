# Semaian System Overview

## Product
Semaian is a coffee ecosystem platform for consumers and coffee-industry participants. Consumer education and brewing guidance are first-class product capabilities; B2B discovery and commerce are progressive capabilities.

## Initial topology
Start with a modular monolith: one Go API deployment with clearly separated domain modules. This reduces cost and operational burden while preserving boundaries for future extraction.

## Clients
- Next.js web
- React Native mobile
- Business/admin experiences may initially be role-based areas of the web application.

## Backend
- Go, net/http, Chi
- PostgreSQL
- OpenAPI contracts
- Docker local development

## Future evolution
Extract a module into a separately deployed service only when scale, team ownership, security, or independent release needs justify it. An extracted service owns its database and migrations. No cross-service SQL joins or shared database credentials.
