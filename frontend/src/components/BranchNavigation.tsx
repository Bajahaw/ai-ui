import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import { Button } from "@/components/ui/button";

interface BranchNavigationProps {
  currentIndex: number;
  totalCount: number;
  onPrevious: () => void;
  onNext: () => void;
  className?: string;
}

export const BranchNavigation = ({
  currentIndex,
  totalCount,
  onPrevious,
  onNext,
  className = "",
}: BranchNavigationProps) => {
  if (totalCount <= 1) {
    return null;
  }

  return (
    <div className={`flex items-center gap-1 ${className}`}>
      <Button
        variant="ghost"
        size="sm"
        onClick={onPrevious}
        disabled={currentIndex === 0}
        className="h-6 w-6 p-0 hover:bg-muted"
      >
        <ChevronLeftIcon className="h-3 w-3" />
      </Button>

      <span className="text-xs text-muted-foreground px-1 min-w-fit">
        {currentIndex + 1} / {totalCount}
      </span>

      <Button
        variant="ghost"
        size="sm"
        onClick={onNext}
        disabled={currentIndex === totalCount - 1}
        className="h-6 w-6 p-0 hover:bg-muted"
      >
        <ChevronRightIcon className="h-3 w-3" />
      </Button>
    </div>
  );
};
