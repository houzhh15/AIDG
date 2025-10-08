import React, { useState, useCallback, useRef } from 'react';
import { Input, AutoComplete } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { GlobalSearchBoxProps } from '../../types/documents';

const { Search } = Input;

const GlobalSearchBox: React.FC<GlobalSearchBoxProps> = ({
  projectId,
  onSearch,
  placeholder = "搜索文档内容..."
}) => {
  const [searchValue, setSearchValue] = useState<string>('');
  const [suggestions, setSuggestions] = useState<{ value: string; label: string }[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  // 防抖搜索建议
  const debouncedGetSuggestions = useCallback(
    (query: string) => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
      
      timeoutRef.current = setTimeout(async () => {
        if (query.length < 2) {
          setSuggestions([]);
          return;
        }

        setLoading(true);
        try {
          // 调用搜索建议API
          const { default: documentsAPI } = await import('../../api/documents');
          const result = await documentsAPI.getSearchSuggestions(projectId, query, 10);
          
          // 将API返回的建议转换为AutoComplete需要的格式
          const apiSuggestions = result.suggestions.map(suggestion => ({
            value: suggestion,
            label: suggestion
          }));
          
          setSuggestions(apiSuggestions);
        } catch (error) {
          console.error('获取搜索建议失败:', error);
          setSuggestions([]);
        } finally {
          setLoading(false);
        }
      }, 500);
    },
    [projectId]
  );

  const handleSearchChange = (value: string) => {
    setSearchValue(value);
    debouncedGetSuggestions(value);
  };

  const handleSearch = (value: string) => {
    if (value.trim()) {
      onSearch(value.trim());
    }
  };

  const handleSelect = (value: string) => {
    setSearchValue(value);
    handleSearch(value);
  };

  return (
    <div className="global-search-box" style={{ width: '100%', maxWidth: 400 }}>
      <AutoComplete
        value={searchValue}
        options={suggestions}
        onSelect={handleSelect}
        onSearch={handleSearchChange}
        style={{ width: '100%' }}
      >
        <Search
          placeholder={placeholder}
          enterButton={<SearchOutlined />}
          loading={loading}
          onSearch={handleSearch}
          allowClear
        />
      </AutoComplete>
    </div>
  );
};

export default GlobalSearchBox;