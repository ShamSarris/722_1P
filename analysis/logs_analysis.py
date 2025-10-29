import csv
import statistics
import glob
import os

RESULTS_DIR = "../benchmark/latency_logs/"

def read_latency_csv(filepath):
    """Read latency values from a CSV file."""
    latencies = []
    with open(filepath, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            # Assuming CSV has a 'latency' column with numeric values
            latencies.append(float(row['Latency (ms)']))
    return latencies

def calculate_stats(latencies):
    """Calculate average and 99th percentile latency."""
    if not latencies:
        return None, None, None
    
    avg_latency = statistics.mean(latencies)
    percentile_99 = statistics.quantiles(latencies, n=100)[98]  # 99th percentile
    percentile_99_999 = statistics.quantiles(latencies, n=100000)[99998]  # 99.999th percentile
    
    return avg_latency, percentile_99, percentile_99_999

def analyze_all_logs():
    """Analyze all CSV files in the latency logs directory."""
    csv_pattern = os.path.join(RESULTS_DIR, "*.csv")
    csv_files = glob.glob(csv_pattern)
    
    if not csv_files:
        print(f"No CSV files found in {RESULTS_DIR}")
        return
    
    print("Latency Analysis Results")
    print("=" * 70)
    
    for csv_file in sorted(csv_files):
        filename = os.path.basename(csv_file)
        try:
            latencies = read_latency_csv(csv_file)
            avg, p99, p99_999 = calculate_stats(latencies)
            
            print(f"\nFile: {filename}")
            print(f"  Sample size: {len(latencies)}")
            print(f"  Average latency: {avg:.4f} ms")
            print(f"  99th percentile: {p99:.4f} ms")
            print(f"  99.999th percentile: {p99_999:.4f} ms")
        except Exception as e:
            print(f"\nFile: {filename}")
            print(f"  Error: {str(e)}")
    
    print("\n" + "=" * 70)

if __name__ == "__main__":
    analyze_all_logs()

