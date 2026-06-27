<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet
  version="1.0"
  xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
  xmlns:exsl="http://exslt.org/common"
  extension-element-prefixes="exsl">
  
  <!-- Remove XML formatting whitespace -->
  <xsl:strip-space elements="*"/>
  
  <!-- Gemtext is plain text -->
  <xsl:output method="text" encoding="UTF-8"/>
  
  <!-- Root -->
  <xsl:template match="/document">
    <xsl:apply-templates select="body/*"/>
  </xsl:template>
  
  <!-- BOLD -->
  <xsl:template match="bold">
    <xsl:text>&#10;</xsl:text> <!-- single line before bold -->
    <xsl:text>### </xsl:text>
    <xsl:value-of select="."/>
    <xsl:text>&#10;</xsl:text> 
  </xsl:template>
  
  <!-- CODE -->
  <xsl:template match="code">
    
    <xsl:choose>
      <xsl:when test="table">
        <xsl:apply-templates select="table"/>
      </xsl:when>
      <xsl:otherwise>
        <xsl:text>&#10;</xsl:text> <!-- ensure single line before code -->
        <xsl:text>```&#10;</xsl:text>
        <xsl:value-of select="normalize-space(.)"/>
        <xsl:text>&#10;```&#10;</xsl:text>
      </xsl:otherwise>
    </xsl:choose>
    
  </xsl:template>
  
  <xsl:template match="table">
    <xsl:variable name="tableNode" select="descendant-or-self::table"/>
    <xsl:if test="$tableNode">
      <xsl:text>```&#10;</xsl:text>
      <xsl:variable name="colCount" select="count($tableNode//tr[1]/*[self::td or self::th])"/>
      
      <xsl:call-template name="draw-border">
        <xsl:with-param name="table" select="$tableNode"/>
        <xsl:with-param name="colIndex" select="1"/>
        <xsl:with-param name="totalCols" select="$colCount"/>
      </xsl:call-template>
      
      <xsl:for-each select="$tableNode//tr">
        <xsl:call-template name="render-row">
          <xsl:with-param name="table" select="$tableNode"/>
          <xsl:with-param name="totalCols" select="$colCount"/>
        </xsl:call-template>
      </xsl:for-each>
      <xsl:text>```&#10;</xsl:text>
    </xsl:if>
  </xsl:template>
  
  <xsl:template name="render-row">
    <xsl:param name="table"/>
    <xsl:param name="totalCols"/>
    <xsl:text>|</xsl:text>
    <xsl:for-each select="*[self::td or self::th]">
      <xsl:variable name="pos" select="position()"/>
      <xsl:variable name="width">
        <xsl:call-template name="get-max-width">
          <xsl:with-param name="nodes" select="$table//tr/*[position() = $pos]"/>
        </xsl:call-template>
      </xsl:variable>
      
      <xsl:variable name="cellText" select="normalize-space(.)"/>
      <xsl:text> </xsl:text>
      <xsl:value-of select="$cellText"/>
      <xsl:call-template name="repeat-char">
        <xsl:with-param name="count" select="$width - string-length($cellText) + 1"/>
      </xsl:call-template>
      <xsl:text>|</xsl:text>
    </xsl:for-each>
    <xsl:text>&#10;</xsl:text>
    
    <xsl:call-template name="draw-border">
      <xsl:with-param name="table" select="$table"/>
      <xsl:with-param name="colIndex" select="1"/>
      <xsl:with-param name="totalCols" select="$totalCols"/>
    </xsl:call-template>
  </xsl:template>
  
  <xsl:template name="draw-border">
    <xsl:param name="table"/>
    <xsl:param name="colIndex"/>
    <xsl:param name="totalCols"/>
    <xsl:if test="$colIndex = 1"><xsl:text>+</xsl:text></xsl:if>
    
    <xsl:variable name="width">
      <xsl:call-template name="get-max-width">
        <xsl:with-param name="nodes" select="$table//tr/*[position() = $colIndex]"/>
      </xsl:call-template>
    </xsl:variable>
    
    <xsl:call-template name="repeat-char">
      <xsl:with-param name="count" select="$width + 2"/>
      <xsl:with-param name="char" select="'-'"/>
    </xsl:call-template>
    <xsl:text>+</xsl:text>
    
    <xsl:if test="$colIndex &lt; $totalCols">
      <xsl:call-template name="draw-border">
        <xsl:with-param name="table" select="$table"/>
        <xsl:with-param name="colIndex" select="$colIndex + 1"/>
        <xsl:with-param name="totalCols" select="$totalCols"/>
      </xsl:call-template>
    </xsl:if>
    <xsl:if test="$colIndex = $totalCols"><xsl:text>&#10;</xsl:text></xsl:if>
  </xsl:template>
  
  <xsl:template name="get-max-width">
    <xsl:param name="nodes"/>
    <xsl:param name="currentMax" select="0"/>
    <xsl:choose>
      <xsl:when test="$nodes">
        <xsl:variable name="len" select="string-length(normalize-space($nodes[1]))"/>
        <xsl:variable name="newMax">
          <xsl:choose>
            <xsl:when test="$len > $currentMax"><xsl:value-of select="$len"/></xsl:when>
            <xsl:otherwise><xsl:value-of select="$currentMax"/></xsl:otherwise>
          </xsl:choose>
        </xsl:variable>
        <xsl:call-template name="get-max-width">
          <xsl:with-param name="nodes" select="$nodes[position() > 1]"/>
          <xsl:with-param name="currentMax" select="$newMax"/>
        </xsl:call-template>
      </xsl:when>
      <xsl:otherwise><xsl:value-of select="$currentMax"/></xsl:otherwise>
    </xsl:choose>
  </xsl:template>
  
  <xsl:template name="repeat-char">
    <xsl:param name="count"/>
    <xsl:param name="char" select="' '"/>
    <xsl:if test="$count > 0">
      <xsl:value-of select="$char"/>
      <xsl:call-template name="repeat-char">
        <xsl:with-param name="count" select="$count - 1"/>
        <xsl:with-param name="char" select="$char"/>
      </xsl:call-template>
    </xsl:if>
  </xsl:template>
  
  <!-- LIST -->
  <xsl:template match="item[not(preceding-sibling::*[1][self::item])]">
    <xsl:text>&#10;</xsl:text> <!-- single blank line before list -->
    <xsl:apply-templates
      select=". | following-sibling::*[
          self::item
          and
          generate-id(preceding-sibling::*[not(self::item)][1]) 
          = generate-id(current()/preceding-sibling::*[not(self::item)][1])
        ]"
      mode="item-group"/>
  </xsl:template>
  
  <xsl:template match="item"/>
  
  <xsl:template match="item" mode="item-group">
    <xsl:text>* </xsl:text>
    <xsl:value-of select="."/>
    <xsl:text>&#10;</xsl:text>
  </xsl:template>
  
  <!-- LINK -->
  <xsl:template match="link[not(preceding-sibling::*[1][self::link])]">
    <xsl:text>&#10;</xsl:text> <!-- blank line before link group -->
    <xsl:apply-templates
      select=". | following-sibling::*[
          self::link
          and
          generate-id(preceding-sibling::*[not(self::link)][1]) 
          = generate-id(current()/preceding-sibling::*[not(self::link)][1])
        ]"
      mode="link-group"/>
  </xsl:template>
  
  <xsl:template match="link"/>
  
  <xsl:template match="link" mode="link-group">
    <xsl:text>=&gt; </xsl:text>
    <xsl:value-of select="@href"/>
    <xsl:text> </xsl:text>
    <xsl:value-of select="."/>
    <xsl:text>&#10;</xsl:text>
  </xsl:template>
  
  <!-- TEXT -->
  <xsl:template match="text">
    <xsl:variable name="t" select="normalize-space(.)"/>
    <xsl:if test="$t != ''">
      <xsl:text>&#10;</xsl:text>  <!-- single blank line before paragraph -->
      <xsl:value-of select="$t"/>
      <xsl:text>&#10;</xsl:text>
    </xsl:if>
  </xsl:template>
  
</xsl:stylesheet>