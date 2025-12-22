import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "../ui/card";
import { Database, Edit, ExternalLink, Loader2, Plus, RefreshCw, Trash2, } from "lucide-react";
import { ProviderForm } from "./ProviderForm";
import { FrontendProvider, ProviderRequest } from "@/lib/api/types";
import { useSettingsData } from "@/hooks/useSettingsData";
import { useModelsContext } from "@/hooks/useModelsContext";

export const ProvidersSection = () => {
	const { data, addProvider, updateProvider, deleteProvider, getModelsByProvider } = useSettingsData();
	const { refreshModels } = useModelsContext();
	const [showAddForm, setShowAddForm] = useState(false);
	const [editingProvider, setEditingProvider] = useState<FrontendProvider | null>(null);
	const [loadingModels, setLoadingModels] = useState(false);

	const handleAddProvider = async (providerData: ProviderRequest) => {
		await addProvider(providerData);
		// addProvider already refreshes models internally
		setShowAddForm(false);
	};

	const handleEditProvider = async (providerData: ProviderRequest) => {
		if (editingProvider) {
			await updateProvider(providerData);
			setEditingProvider(null);
		}
	};

	const handleDeleteProvider = async (id: string) => {
		if (confirm("Are you sure you want to delete this provider?")) {
			await deleteProvider(id);
		}
	};

	const handleRefreshModels = async () => {
		setLoadingModels(true);
		try {
			await refreshModels();
		} finally {
			setLoadingModels(false);
		}
	};

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<h3 className="text-lg font-medium flex items-center gap-2">
					<Database className="h-5 w-5" />
					AI Providers
				</h3>
				<div className="flex items-center gap-2">
					<Button onClick={handleRefreshModels} variant="ghost" size="sm" disabled={loadingModels}>
						{loadingModels ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
					</Button>
					<Button onClick={() => setShowAddForm(true)} variant="outline" size="sm">
						<Plus className="h-4 w-4" />
						<span className="hidden sm:inline">Add Provider</span>
					</Button>
				</div>
			</div>

			{data.providers.length === 0 ? (
				<Card className="p-6 text-center bg-transparent border-dashed">
					<div className="space-y-2">
						<p className="text-muted-foreground">No providers configured</p>
						<Button
							onClick={() => setShowAddForm(true)}
							variant="outline"
							size="sm"
						>
							<Plus className="h-4 w-4" />
							Add Your First Provider
						</Button>
					</div>
				</Card>
			) : (
				<div className="space-y-4 overflow-hidden">
					{data.providers.map((provider) => {
						const providerModels = getModelsByProvider(provider.id);
						return (
							<Card
								key={provider.id}
								className="p-4 bg-transparent overflow-hidden"
							>
								<div className="space-y-3">
									<div className="flex items-start justify-between gap-4">
										<div className="flex-1 min-w-0">
										<h4 className="truncate max-w-[75px] sm:max-w-[300px]">
												{provider.id}
											</h4>
										</div>

										<div className="flex items-center gap-1 flex-shrink-0">
											<Button
												variant="ghost"
												size="sm"
												onClick={() => setEditingProvider(provider)}
												title="Edit provider"
											>
												<Edit className="h-4 w-4" />
											</Button>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => handleDeleteProvider(provider.id)}
												className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20"
												title="Delete provider"
											>
												<Trash2 className="h-4 w-4" />
											</Button>
										</div>
									</div>

									<div className="flex items-center gap-2 text-sm text-muted-foreground">
										<ExternalLink className="h-3 w-3 flex-shrink-0" />
										<span
											className="truncate max-w-[175px] sm:max-w-[300px]"
											title={provider.baseUrl}
										>
											{provider.baseUrl}
										</span>
									</div>

									{providerModels.length > 0 && (
										<div className="flex flex-wrap gap-1 overflow-hidden">
											{providerModels.slice(0, 5).map((model) => (
												<Badge
													key={model.id}
													variant="outline"
													className="text-xs truncate max-w-[120px]"
													title={model.name}
												>
													{model.name}
												</Badge>
											))}
											{providerModels.length > 5 && (
												<Badge variant="outline" className="text-xs">
													+{providerModels.length - 5} more
												</Badge>
											)}
										</div>
									)}
								</div>
							</Card>
						)
					})}
				</div>
			)}

			{/* Add Provider Form */}
			<ProviderForm
				open={showAddForm}
				onOpenChange={setShowAddForm}
				onSubmit={handleAddProvider}
				title="Add AI Provider"
				submitLabel="Add Provider"
			/>

			{/* Edit Provider Form */}
			<ProviderForm
				open={!!editingProvider}
				onOpenChange={(open) => !open && setEditingProvider(null)}
				onSubmit={handleEditProvider}
				provider={editingProvider}
				title="Edit AI Provider"
				submitLabel="Update Provider"
			/>
		</div>
	);
};
