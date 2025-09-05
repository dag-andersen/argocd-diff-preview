# .github/scripts/summarize.py
import os
import sys
import numpy as np


def format_duration(seconds):
    """Formats seconds into a human-readable m s string."""
    seconds = int(seconds)
    minutes = seconds // 60
    remaining_seconds = seconds % 60
    return f"{minutes}m {remaining_seconds}s"


def main():
    if len(sys.argv) < 2:
        print("Usage: python summarize.py <durations_directory>")
        sys.exit(1)

    durations_dir = sys.argv[1]
    durations = []

    if not os.path.isdir(durations_dir):
        print(f"Error: Directory '{durations_dir}' not found.")
        sys.exit(1)

    for filename in os.listdir(durations_dir):
        if filename.endswith(".txt"):
            try:
                with open(os.path.join(durations_dir, filename), "r") as f:
                    duration = int(f.read().strip())
                    durations.append(duration)
            except (ValueError, IOError) as e:
                print(f"Warning: Could not read or parse {filename}. Error: {e}")

    if not durations:
        print("No valid duration files found. Cannot generate a summary.")
        return

    # Calculate statistics
    total_runs = len(durations)
    total_time = np.sum(durations)
    avg_time = np.mean(durations)
    median_time = np.median(durations)
    fastest_time = np.min(durations)
    slowest_time = np.max(durations)
    p90_time = np.percentile(durations, 90)
    p99_time = np.percentile(durations, 99)

    # Generate Markdown summary
    summary_md = f"""
## ⚡ Integration Test Performance Summary

A total of **{total_runs}** parallel integration test suites were completed.

| Metric | Duration |
|---|---|
|  slowest (p100) | {format_duration(slowest_time)} |
| p99 | {format_duration(p99_time)} |
| p90 | {format_duration(p90_time)} |
| **Median (p50)** | **{format_duration(median_time)}** |
| Average | {format_duration(avg_time)} |
| Fastest (p0) | {format_duration(fastest_time)} |
| **Total Time** | **{format_duration(total_time)}** |
"""
    print(summary_md.strip())


if __name__ == "__main__":
    main()
