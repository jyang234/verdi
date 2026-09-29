---
id: spec/real-use-checkpoint
kind: spec
title: "Real-use checkpoint"
owners: [platform-team]
class: story
story: jira:VERDI-WR-11
problem: { text: "No one has yet completed a journey on the workbench without the development agent's help, and no journey has ever been timed. The redesign is judged by whether that changes.", anchor: problem }
outcome: { text: "A person new to the store works two comparable real changes on the redesigned workbench under a protocol written beforehand, and the record shows how long each took, whether the second finished without coaching, and every point where help was needed.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Before the first journey begins, a written protocol fixes where each journey starts and ends, what assistance is permitted, and how duration is measured, and names the two real changes.", evidence: [attestation], anchor: ac-1 }
  - { id: ac-2, text: "A person new to the store works two comparable real changes on the redesigned workbench, each defining a change, reaching agreement on it, and following it; the second is a new change, not a replay of the first.", evidence: [attestation], anchor: ac-2 }
  - { id: ac-3, text: "The record of each journey reports its duration without a threshold chosen afterwards, whether the person finished independently, and every point where assistance was needed, and the second journey finishes without coaching from the development agent.", evidence: [attestation], anchor: ac-3 }
constraints:
  - { id: co-1, text: "The journeys run on one release that carries every other redesign story, and no workbench change is made between the two journeys.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "This story builds nothing. It is the owner's checkpoint (SI-199's local MVP; SI-286), and its evidence is the owner's attestation of the protocol and the two records.", anchor: dc-1 }
  - { id: dc-2, text: "The protocol is committed to the repository before the first journey begins, and each journey's record after it ends, so what was fixed beforehand can be checked against what happened.", anchor: dc-2 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-8" }
---
# Real-use checkpoint

## Problem

No one has yet completed a journey on the workbench without the development agent's help, and no journey has ever been
timed. The redesign is judged by whether that changes.

## Outcome

A person new to the store works two comparable real changes on the redesigned workbench under a protocol written
beforehand, and the record shows how long each took, whether the second finished without coaching, and every point where
help was needed.

## ac-1

Before the first journey begins, a written protocol fixes where each journey starts and ends, what assistance is
permitted, and how duration is measured, and names the two real changes.

## ac-2

A person new to the store works two comparable real changes on the redesigned workbench, each defining a change,
reaching agreement on it, and following it; the second is a new change, not a replay of the first.

## ac-3

The record of each journey reports its duration without a threshold chosen afterwards, whether the person finished
independently, and every point where assistance was needed, and the second journey finishes without coaching from the
development agent.

## co-1

The journeys run on one release that carries every other redesign story, and no workbench change is made between the two
journeys.

## dc-1

This story builds nothing. It is the owner's checkpoint (SI-199's local MVP; SI-286), and its evidence is the owner's
attestation of the protocol and the two records.

## dc-2

The protocol is committed to the repository before the first journey begins, and each journey's record after it ends, so
what was fixed beforehand can be checked against what happened.
