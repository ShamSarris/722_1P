import csv
import glob
import os
import matplotlib.pyplot as plt
import statistics

RESULTS_DIR = "../benchmark/latency_logs/"

def extract_throughput_from_filename(filename):
    """Extract throughput value from filename (e.g., 'latency_100rps.csv' -> 100)."""
    try:
        # Assuming format like 'latency_XXXrps.csv' or similar
        parts = filename.replace('.csv', '').split('_')
        for part in parts:
            if 'rps' in part.lower():
                return int(part.lower().replace('rps', ''))
    except:
        pass
    return None

def read_latency_csv(filepath):
    """Read latency values from a CSV file."""
    latencies = []
    with open(filepath, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            latencies.append(float(row['Latency (ms)']))
    return latencies

def calculate_avg_latency(latencies):
    """Calculate average latency."""
    return statistics.mean(latencies) if latencies else None

def plot_throughput_vs_latency():
    """Create a plot mapping throughput to average latency."""
    csv_pattern = os.path.join(RESULTS_DIR, "*.csv")
    csv_files = glob.glob(csv_pattern)
    
    if not csv_files:
        print(f"No CSV files found in {RESULTS_DIR}")
        return
    
    data_points = []
    
    for csv_file in csv_files:
        filename = os.path.basename(csv_file)
        throughput = extract_throughput_from_filename(filename)
        
        if throughput is None:
            print(f"Warning: Could not extract throughput from {filename}")
            continue
        
        try:
            latencies = read_latency_csv(csv_file)
            avg_latency = calculate_avg_latency(latencies)
            
            if avg_latency is not None:
                data_points.append((throughput, avg_latency))
        except Exception as e:
            print(f"Error processing {filename}: {str(e)}")
    
    if not data_points:
        print("No valid data points to plot")
        return
    
    # Sort by throughput
    data_points.sort(key=lambda x: x[0])
    throughputs, latencies = zip(*data_points)
    
    # Create the plot
    plt.figure(figsize=(10, 6))
    plt.plot(throughputs, latencies, marker='o', linestyle='-', linewidth=2, markersize=8)
    plt.xlabel('Throughput (requests/second)', fontsize=12)
    plt.ylabel('Average Latency (ms)', fontsize=12)
    plt.title('Throughput vs Average Latency', fontsize=14, fontweight='bold')
    plt.grid(True, alpha=0.3)
    plt.tight_layout()
    
    # Save the plot
    output_path = os.path.join(RESULTS_DIR, "throughput_vs_latency.png")
    plt.savefig(output_path, dpi=300)
    print(f"Plot saved to {output_path}")
    
    plt.show()

if __name__ == "__main__":
    plot_throughput_vs_latency()