# Pitfalls

This directory documents common mistakes and anti-patterns discovered during development. Each file describes a specific pitfall, its root cause, symptoms, and prevention.

## Purpose

- **Learn from past mistakes**: Document issues that caused bugs or confusion
- **Prevent recurrence**: Reference during code review to catch similar patterns
- **Knowledge transfer**: Help new contributors avoid known traps

## Reading Guide

When reviewing code or implementing new features, scan these documents for:
- Similar API usage patterns
- Related data flow paths
- Analogous encoding/decoding scenarios

## Contents

- [double-base64-encoding.md](double-base64-encoding.md) — Pre-encoding data before calling APIs that expect raw bytes