package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"clara-agents/internal/e2b"
)

// NewMLTrainerTool creates a new ML Model Trainer tool
func NewMLTrainerTool() *Tool {
	return &Tool{
		Name:        "train_model",
		DisplayName: "ML Model Trainer",
		Description: "Train machine learning models on your data using scikit-learn. Supports classification (predict categories), regression (predict numbers), and clustering (find groups). Automatically handles data preprocessing, trains the model, evaluates performance, and returns metrics with feature importance visualizations.",
		Icon:        "Brain",
		Source:      ToolSourceBuiltin,
		Category:    "computation",
		Keywords:    []string{"machine learning", "ML", "train", "model", "predict", "classification", "regression", "clustering", "scikit-learn", "AI"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_id": map[string]interface{}{
					"type":        "string",
					"description": "Upload ID of the CSV/Excel file containing training data (from file upload). Use this for uploaded files.",
				},
				"csv_data": map[string]interface{}{
					"type":        "string",
					"description": "CSV data as a string (with headers). Use this for small inline data. Either file_id or csv_data is required.",
				},
				"task_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of machine learning task",
					"enum":        []string{"classification", "regression", "clustering"},
				},
				"target_column": map[string]interface{}{
					"type":        "string",
					"description": "Name of the column to predict (required for classification and regression, not used for clustering)",
				},
				"model_type": map[string]interface{}{
					"type":        "string",
					"description": "Specific model algorithm to use (optional, will auto-select if not provided)",
					"enum":        []string{"random_forest", "logistic_regression", "linear_regression", "decision_tree", "kmeans", "dbscan"},
				},
				"test_size": map[string]interface{}{
					"type":        "number",
					"description": "Fraction of data to use for testing (default: 0.2)",
					"default":     0.2,
					"minimum":     0.1,
					"maximum":     0.5,
				},
			},
			"required": []string{"task_type"},
		},
		Execute: executeMLTrainer,
	}
}

func executeMLTrainer(args map[string]interface{}) (string, error) {
	var csvData []byte
	var filename string

	// Try file_id first (uploaded files)
	if fileID, ok := args["file_id"].(string); ok && fileID != "" {
		content, name, err := GetUploadedFile(fileID)
		if err != nil {
			return "", fmt.Errorf("failed to get uploaded file: %w", err)
		}
		csvData = content
		filename = name
	} else if csvDataStr, ok := args["csv_data"].(string); ok && csvDataStr != "" {
		// Fallback to direct CSV data for small inline data
		csvData = []byte(csvDataStr)
		filename = "data.csv"
	} else {
		return "", fmt.Errorf("either file_id or csv_data is required")
	}

	taskType, ok := args["task_type"].(string)
	if !ok {
		return "", fmt.Errorf("task_type must be a string")
	}

	targetColumn := ""
	if taskType != "clustering" {
		tc, ok := args["target_column"].(string)
		if !ok || tc == "" {
			return "", fmt.Errorf("target_column is required for %s tasks", taskType)
		}
		targetColumn = tc
	}

	modelType := ""
	if mt, ok := args["model_type"].(string); ok {
		modelType = mt
	}

	testSize := 0.2
	if ts, ok := args["test_size"].(float64); ok {
		testSize = ts
	}

	// Create file map with CSV data
	files := map[string][]byte{
		filename: csvData,
	}

	// Generate Python code
	pythonCode := generateMLTrainingCode(filename, taskType, targetColumn, modelType, testSize)

	// Execute code
	e2bService := e2b.GetE2BExecutorService()
	result, err := e2bService.ExecuteWithFiles(context.Background(), pythonCode, files, 120) // 2 minutes timeout
	if err != nil {
		return "", fmt.Errorf("failed to execute training: %w", err)
	}

	if !result.Success {
		if result.Error != nil {
			return "", fmt.Errorf("training failed: %s", *result.Error)
		}
		return "", fmt.Errorf("training failed with stderr: %s", result.Stderr)
	}

	// Format response
	response := map[string]interface{}{
		"success":    true,
		"task_type":  taskType,
		"model_type": modelType,
		"output":     result.Stdout,
		"plots":      result.Plots,
		"plot_count": len(result.Plots),
	}

	jsonResponse, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResponse), nil
}

func generateMLTrainingCode(fileName, taskType, targetColumn, modelType string, testSize float64) string {
	baseCode := fmt.Sprintf(`import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import LabelEncoder, StandardScaler
from sklearn.metrics import classification_report, confusion_matrix, mean_squared_error, r2_score
import warnings
warnings.filterwarnings('ignore')

# Set plot style
plt.style.use('seaborn-v0_8-darkgrid')
sns.set_palette("husl")

print("=" * 80)
print("🤖 MACHINE LEARNING MODEL TRAINER")
print("=" * 80)

# Load data
print(f"\n📁 Loading data from: %s")
df = pd.read_csv('%s')
print(f"✅ Loaded {df.shape[0]} rows × {df.shape[1]} columns")
`, fileName, fileName)

	switch taskType {
	case "classification":
		baseCode += generateClassificationCode(targetColumn, modelType, testSize)
	case "regression":
		baseCode += generateRegressionCode(targetColumn, modelType, testSize)
	case "clustering":
		baseCode += generateClusteringCode(modelType)
	}

	baseCode += `
print("\n" + "=" * 80)
print("✅ TRAINING COMPLETE")
print("=" * 80)
`
	return baseCode
}

func generateClassificationCode(targetColumn, modelType string, testSize float64) string {
	model := "RandomForestClassifier(n_estimators=100, random_state=42)"
	modelName := "Random Forest"

	if modelType == "logistic_regression" {
		model = "LogisticRegression(max_iter=1000, random_state=42)"
		modelName = "Logistic Regression"
	} else if modelType == "decision_tree" {
		model = "DecisionTreeClassifier(random_state=42)"
		modelName = "Decision Tree"
	}

	return fmt.Sprintf(`
from sklearn.ensemble import RandomForestClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.tree import DecisionTreeClassifier

# Prepare data
print(f"\n🎯 Target column: %s")
print(f"📊 Task: Classification")
print(f"🤖 Model: %s")

# Check if target column exists
if '%s' not in df.columns:
    raise ValueError(f"Target column '%s' not found. Available columns: {list(df.columns)}")

# Separate features and target
X = df.drop('%s', axis=1)
y = df['%s']

# Handle categorical variables
categorical_cols = X.select_dtypes(include=['object']).columns
if len(categorical_cols) > 0:
    print(f"\n🔤 Encoding {len(categorical_cols)} categorical columns...")
    for col in categorical_cols:
        le = LabelEncoder()
        X[col] = le.fit_transform(X[col].astype(str))

# Encode target if categorical
if y.dtype == 'object':
    le = LabelEncoder()
    y = le.fit_transform(y)
    print(f"✅ Target encoded: {len(le.classes_)} classes")

# Split data
X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=%.2f, random_state=42)
print(f"\n📊 Training set: {len(X_train)} samples")
print(f"📊 Test set: {len(X_test)} samples")

# Scale features
scaler = StandardScaler()
X_train_scaled = scaler.fit_transform(X_train)
X_test_scaled = scaler.transform(X_test)

# Train model
print(f"\n🏋️  Training %s...")
model = %s
model.fit(X_train_scaled, y_train)
print("✅ Model trained!")

# Evaluate
y_pred = model.predict(X_test_scaled)
accuracy = model.score(X_test_scaled, y_test)

print(f"\n📈 RESULTS")
print("-" * 80)
print(f"Accuracy: {accuracy:.4f} ({accuracy*100:.2f}%%)")

print(f"\n📊 Classification Report:")
print(classification_report(y_test, y_pred))

# Confusion Matrix
cm = confusion_matrix(y_test, y_pred)
plt.figure(figsize=(8, 6))
sns.heatmap(cm, annot=True, fmt='d', cmap='Blues', cbar=True)
plt.title(f'Confusion Matrix - %s', fontsize=14, fontweight='bold')
plt.ylabel('True Label')
plt.xlabel('Predicted Label')
plt.tight_layout()
plt.show()

# Feature Importance
if hasattr(model, 'feature_importances_'):
    feature_importance = pd.DataFrame({
        'feature': X.columns,
        'importance': model.feature_importances_
    }).sort_values('importance', ascending=False)

    print(f"\n🔍 Top 10 Most Important Features:")
    print(feature_importance.head(10).to_string(index=False))

    # Plot feature importance
    plt.figure(figsize=(10, 6))
    top_features = feature_importance.head(15)
    plt.barh(range(len(top_features)), top_features['importance'])
    plt.yticks(range(len(top_features)), top_features['feature'])
    plt.xlabel('Importance')
    plt.title('Top 15 Feature Importance', fontsize=14, fontweight='bold')
    plt.gca().invert_yaxis()
    plt.tight_layout()
    plt.show()
`, targetColumn, modelName, targetColumn, targetColumn, targetColumn, targetColumn, testSize, modelName, model, modelName)
}

func generateRegressionCode(targetColumn, modelType string, testSize float64) string {
	model := "RandomForestRegressor(n_estimators=100, random_state=42)"
	modelName := "Random Forest"

	if modelType == "linear_regression" {
		model = "LinearRegression()"
		modelName = "Linear Regression"
	} else if modelType == "decision_tree" {
		model = "DecisionTreeRegressor(random_state=42)"
		modelName = "Decision Tree"
	}

	return fmt.Sprintf(`
from sklearn.ensemble import RandomForestRegressor
from sklearn.linear_model import LinearRegression
from sklearn.tree import DecisionTreeRegressor

# Prepare data
print(f"\n🎯 Target column: %s")
print(f"📊 Task: Regression")
print(f"🤖 Model: %s")

# Check if target column exists
if '%s' not in df.columns:
    raise ValueError(f"Target column '%s' not found. Available columns: {list(df.columns)}")

# Separate features and target
X = df.drop('%s', axis=1)
y = df['%s']

# Handle categorical variables
categorical_cols = X.select_dtypes(include=['object']).columns
if len(categorical_cols) > 0:
    print(f"\n🔤 Encoding {len(categorical_cols)} categorical columns...")
    for col in categorical_cols:
        le = LabelEncoder()
        X[col] = le.fit_transform(X[col].astype(str))

# Split data
X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=%.2f, random_state=42)
print(f"\n📊 Training set: {len(X_train)} samples")
print(f"📊 Test set: {len(X_test)} samples")

# Scale features
scaler = StandardScaler()
X_train_scaled = scaler.fit_transform(X_train)
X_test_scaled = scaler.transform(X_test)

# Train model
print(f"\n🏋️  Training %s...")
model = %s
model.fit(X_train_scaled, y_train)
print("✅ Model trained!")

# Evaluate
y_pred = model.predict(X_test_scaled)
mse = mean_squared_error(y_test, y_pred)
rmse = np.sqrt(mse)
r2 = r2_score(y_test, y_pred)

print(f"\n📈 RESULTS")
print("-" * 80)
print(f"R² Score: {r2:.4f}")
print(f"RMSE: {rmse:.4f}")
print(f"MSE: {mse:.4f}")

# Prediction vs Actual plot
plt.figure(figsize=(10, 6))
plt.scatter(y_test, y_pred, alpha=0.6)
plt.plot([y_test.min(), y_test.max()], [y_test.min(), y_test.max()], 'r--', lw=2)
plt.xlabel('Actual Values')
plt.ylabel('Predicted Values')
plt.title('Predictions vs Actual Values - %s', fontsize=14, fontweight='bold')
plt.grid(True, alpha=0.3)
plt.tight_layout()
plt.show()

# Residuals plot
residuals = y_test - y_pred
plt.figure(figsize=(10, 6))
plt.scatter(y_pred, residuals, alpha=0.6)
plt.axhline(y=0, color='r', linestyle='--', lw=2)
plt.xlabel('Predicted Values')
plt.ylabel('Residuals')
plt.title('Residual Plot', fontsize=14, fontweight='bold')
plt.grid(True, alpha=0.3)
plt.tight_layout()
plt.show()

# Feature Importance
if hasattr(model, 'feature_importances_'):
    feature_importance = pd.DataFrame({
        'feature': X.columns,
        'importance': model.feature_importances_
    }).sort_values('importance', ascending=False)

    print(f"\n🔍 Top 10 Most Important Features:")
    print(feature_importance.head(10).to_string(index=False))

    # Plot feature importance
    plt.figure(figsize=(10, 6))
    top_features = feature_importance.head(15)
    plt.barh(range(len(top_features)), top_features['importance'])
    plt.yticks(range(len(top_features)), top_features['feature'])
    plt.xlabel('Importance')
    plt.title('Top 15 Feature Importance', fontsize=14, fontweight='bold')
    plt.gca().invert_yaxis()
    plt.tight_layout()
    plt.show()
`, targetColumn, modelName, targetColumn, targetColumn, targetColumn, targetColumn, testSize, modelName, model, modelName)
}

func generateClusteringCode(modelType string) string {
	if modelType == "dbscan" {
		return `
from sklearn.cluster import DBSCAN

print(f"\n📊 Task: Clustering (DBSCAN)")
print(f"🤖 Model: DBSCAN")

# Prepare data
X = df.select_dtypes(include=[np.number])

# Handle categorical variables
categorical_cols = df.select_dtypes(include=['object']).columns
if len(categorical_cols) > 0:
    print(f"\n🔤 Encoding {len(categorical_cols)} categorical columns...")
    for col in categorical_cols:
        le = LabelEncoder()
        X[col] = le.fit_transform(df[col].astype(str))

print(f"\n📊 Features: {X.shape[1]} columns, {X.shape[0]} samples")

# Scale features
scaler = StandardScaler()
X_scaled = scaler.fit_transform(X)

# Train model
print(f"\n🏋️  Training DBSCAN...")
model = DBSCAN(eps=0.5, min_samples=5)
labels = model.fit_predict(X_scaled)
print("✅ Clustering complete!")

# Results
n_clusters = len(set(labels)) - (1 if -1 in labels else 0)
n_noise = list(labels).count(-1)

print(f"\n📈 RESULTS")
print("-" * 80)
print(f"Number of clusters: {n_clusters}")
print(f"Number of noise points: {n_noise}")

# Cluster visualization (PCA)
from sklearn.decomposition import PCA

pca = PCA(n_components=2)
X_pca = pca.fit_transform(X_scaled)

plt.figure(figsize=(10, 6))
scatter = plt.scatter(X_pca[:, 0], X_pca[:, 1], c=labels, cmap='viridis', alpha=0.6)
plt.colorbar(scatter, label='Cluster')
plt.xlabel('First Principal Component')
plt.ylabel('Second Principal Component')
plt.title('DBSCAN Clustering Results (PCA Projection)', fontsize=14, fontweight='bold')
plt.grid(True, alpha=0.3)
plt.tight_layout()
plt.show()

# Cluster sizes
unique, counts = np.unique(labels[labels != -1], return_counts=True)
print(f"\n🔍 Cluster Sizes:")
for cluster_id, count in zip(unique, counts):
    print(f"  Cluster {cluster_id}: {count} samples")
`
	}

	// Default: K-Means
	return `
from sklearn.cluster import KMeans

print(f"\n📊 Task: Clustering (K-Means)")
print(f"🤖 Model: K-Means")

# Prepare data
X = df.select_dtypes(include=[np.number])

# Handle categorical variables
categorical_cols = df.select_dtypes(include=['object']).columns
if len(categorical_cols) > 0:
    print(f"\n🔤 Encoding {len(categorical_cols)} categorical columns...")
    for col in categorical_cols:
        le = LabelEncoder()
        X[col] = le.fit_transform(df[col].astype(str))

print(f"\n📊 Features: {X.shape[1]} columns, {X.shape[0]} samples")

# Scale features
scaler = StandardScaler()
X_scaled = scaler.fit_transform(X)

# Find optimal k using elbow method
print(f"\n🔍 Finding optimal number of clusters...")
inertias = []
K_range = range(2, min(11, len(X) // 2))
for k in K_range:
    kmeans = KMeans(n_clusters=k, random_state=42, n_init=10)
    kmeans.fit(X_scaled)
    inertias.append(kmeans.inertia_)

# Plot elbow curve
plt.figure(figsize=(10, 6))
plt.plot(K_range, inertias, 'bo-', linewidth=2, markersize=8)
plt.xlabel('Number of Clusters (k)')
plt.ylabel('Inertia')
plt.title('Elbow Method for Optimal k', fontsize=14, fontweight='bold')
plt.grid(True, alpha=0.3)
plt.tight_layout()
plt.show()

# Train model with optimal k (using elbow method heuristic)
optimal_k = 3  # Default
if len(inertias) > 1:
    # Simple heuristic: find the elbow point
    diffs = np.diff(inertias)
    optimal_k = K_range[np.argmin(diffs[1:]) + 1] if len(diffs) > 1 else 3

print(f"\n🏋️  Training K-Means with k={optimal_k}...")
model = KMeans(n_clusters=optimal_k, random_state=42, n_init=10)
labels = model.fit_predict(X_scaled)
print("✅ Clustering complete!")

# Results
print(f"\n📈 RESULTS")
print("-" * 80)
print(f"Number of clusters: {optimal_k}")
print(f"Inertia: {model.inertia_:.2f}")

# Cluster visualization (PCA)
from sklearn.decomposition import PCA

pca = PCA(n_components=2)
X_pca = pca.fit_transform(X_scaled)

plt.figure(figsize=(10, 6))
scatter = plt.scatter(X_pca[:, 0], X_pca[:, 1], c=labels, cmap='viridis', alpha=0.6)
plt.scatter(
    pca.transform(model.cluster_centers_)[:, 0],
    pca.transform(model.cluster_centers_)[:, 1],
    c='red', marker='X', s=200, edgecolors='black', label='Centroids'
)
plt.colorbar(scatter, label='Cluster')
plt.xlabel('First Principal Component')
plt.ylabel('Second Principal Component')
plt.title('K-Means Clustering Results (PCA Projection)', fontsize=14, fontweight='bold')
plt.legend()
plt.grid(True, alpha=0.3)
plt.tight_layout()
plt.show()

# Cluster sizes
unique, counts = np.unique(labels, return_counts=True)
print(f"\n🔍 Cluster Sizes:")
for cluster_id, count in zip(unique, counts):
    print(f"  Cluster {cluster_id}: {count} samples ({count/len(labels)*100:.1f}%)")
`
}
